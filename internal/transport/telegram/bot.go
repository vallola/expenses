package telegram

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/vallola/expenses/internal/config"
	"github.com/vallola/expenses/internal/domain"
	"github.com/vallola/expenses/internal/service"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type Bot struct {
	teleBot *tele.Bot
	service *service.ExpenseService
	aiAgent *AIAgent
	stateManager *StateManager
	log	*zap.Logger
}

func NewBot(token string, svc *service.ExpenseService, cfg *config.Config, log *zap.Logger) (*Bot, error) {
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)

	if err != nil {
		return nil, err
	}

	return &Bot{
		teleBot: b,
		service: svc,
		aiAgent: NewAIAgent(cfg, log),
		stateManager: NewStateManager(),
		log: log,
	}, nil
}

// Helper to parse loose date strings returned by LLMs
func parseDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Now()
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t
		}
	}
	return time.Now()
}

func (b *Bot) Start() {
	b.teleBot.Handle(tele.OnText, b.HandleText)
	b.teleBot.Handle(&tele.Btn{Unique: "confirm_action"}, b.HandleConfirmButton)
	b.teleBot.Handle(&tele.Btn{Unique: "cancel_action"}, b.HandleCancelButton)

	b.teleBot.Start()
}

func (b *Bot) HandleText(c tele.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	userID := sender.ID
	
	userName := sender.Username
	if userName == "" {
		userName = strings.TrimSpace(sender.FirstName + " " + sender.LastName)
	}
	if userName == "" {
		userName = fmt.Sprintf("User_%d", userID)
	}

	if err := b.service.SaveUser(ctx, userID, userName); err != nil {
		log.Printf("Failed to upsert user %d: %v", userID, err)
	}

	text := c.Text()

	action, err := b.aiAgent.ParseMessage(ctx, text)
	if err != nil {
		return c.Send("Не смог распознать сообщение. Попробуйте сформулировать иначе.")
	}

	switch action.Action {
	case "create":
		spentAt := parseDate(action.DateStr)
		expense := &domain.Expense{
			UserID:   userID,
			Amount:   action.Amount,
			Category: action.Category,
			SpentAt:  spentAt,
			Comment:  &action.Comment,
		}

		if _, err := b.service.CreateExpense(ctx, expense); err != nil {
			return c.Send("❌ Ошибка при сохранении траты в базу данных.")
		}
		return c.Send(fmt.Sprintf("✅ Добавлено: **%.2f** в категории '%s' (%s)", expense.Amount, expense.Category, expense.SpentAt.Format("02.01.2006")), tele.ModeMarkdown)

	case "list":
		var startDate, endDate *time.Time
		now := time.Now()

		// Strict regex to extract ONLY the YYYY-MM-DD pattern, ignoring any garbage/token bleed like "激励"
		dateRegex := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

		parseCleanDate := func(raw string) *time.Time {
			match := dateRegex.FindString(raw)
			if match != "" {
				if t, err := time.Parse("2006-01-02", match); err == nil {
					return &t
				}
			}
			return nil
		}

		// Extract StartDate (might contain both dates if the model spilled over)
		if action.StartDateStr != "" {
			matches := dateRegex.FindAllString(action.StartDateStr, -1)
			if len(matches) > 0 {
				if t, err := time.Parse("2006-01-02", matches[0]); err == nil {
					startDate = &t
				}
			}
			if len(matches) > 1 {
				// Model accidentally put the end_date inside the start_date string
				if t, err := time.Parse("2006-01-02", matches[1]); err == nil {
					endDate = &t
				}
			}
		}

		// Extract EndDate if it was provided separately
		if endDate == nil && action.EndDateStr != "" {
			endDate = parseCleanDate(action.EndDateStr)
		}

		// If dates are missing, check user text keywords
		textLower := strings.ToLower(text)
		if startDate == nil || endDate == nil {
			if strings.Contains(textLower, "вчера") {
				y := now.AddDate(0, 0, -1)
				startDate, endDate = &y, &y
			} else if strings.Contains(textLower, "позавчера") {
				y := now.AddDate(0, 0, -2)
				startDate, endDate = &y, &y
			} else if strings.Contains(textLower, "сегодня") {
				startDate, endDate = &now, &now
			}
		}

		// Normalize into absolute day boundaries (00:00:00 to 23:59:59)
		if startDate != nil {
			s := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
			startDate = &s
		}

		if endDate != nil {
			e := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 999999999, endDate.Location())
			endDate = &e
		} else if startDate != nil {
			e := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 23, 59, 59, 999999999, startDate.Location())
			endDate = &e
		}

		filter := domain.ExpenseFilter{
			UserID:    &userID,
			StartDate: startDate,
			EndDate:   endDate,
		}

		expenses, err := b.service.ListExpenses(ctx, filter)
		if err != nil {
			return c.Send("❌ Не удалось получить список трат.")
		}

		if len(expenses) == 0 {
			return c.Send("📭 За указанный период трат не найдено.")
		}

		if action.ResponseType == "summary" {
			var total float64

			for _, exp := range expenses {
				total += exp.Amount
			}

			categoryStr := ""

			if action.Category != "" {
				categoryStr = fmt.Sprintf(" в категории *%s*", action.Category)
			}

			return c.Send(fmt.Sprintf("📊 Всего потрачено%s: **%.2f**", categoryStr, total), tele.ModeMarkdown)
		} else {
			var sb strings.Builder

			if action.Category != "" {
				sb.WriteString(fmt.Sprintf("📋 **Ваши траты в категории *%s*:**\n\n", action.Category))
			} else {
				sb.WriteString("📋 **Ваши траты:**\n\n")
			}

			var total float64
			for counter, exp := range expenses {
				if counter == 30 {
					sb.WriteString(fmt.Sprintf("(и ещё %d записей)\n", len(expenses)-(counter+1)))
					break
				}

				total += exp.Amount
				commentStr := ""

				if exp.Comment != nil {
					if *exp.Comment != "" {
						commentStr = fmt.Sprintf("%s", *exp.Comment)
					}
				}

				if commentStr != "" {
					sb.WriteString(fmt.Sprintf("• %s — **%.2f** (%s) _%s_\n", exp.SpentAt.Format("02.01.2006"), exp.Amount, exp.Category, commentStr))
				} else {
					sb.WriteString(fmt.Sprintf("• %s — **%.2f** (%s)\n", exp.SpentAt.Format("02.01.2006"), exp.Amount, exp.Category))
				}
			}

			sb.WriteString(fmt.Sprintf("\n**Итого: %.2f**", total))

			return c.Send(sb.String(), tele.ModeMarkdown)
		}

	case "update", "delete":
		// Extract date filters to find the target record
		var startDate, endDate *time.Time
		now := time.Now()
		dateRegex := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

		if action.StartDateStr != "" {
			matches := dateRegex.FindAllString(action.StartDateStr, -1)
			if len(matches) > 0 {
				if t, err := time.Parse("2006-01-02", matches[0]); err == nil {
					startDate = &t
				}
			}
			if len(matches) > 1 {
				if t, err := time.Parse("2006-01-02", matches[1]); err == nil {
					endDate = &t
				}
			}
		}

		if endDate == nil && action.EndDateStr != "" {
			match := dateRegex.FindString(action.EndDateStr)
			if match != "" {
				if t, err := time.Parse("2006-01-02", match); err == nil {
					endDate = &t
				}
			}
		}

		// Fallback keywords if AI didn't catch dates
		textLower := strings.ToLower(text)
		if startDate == nil {
			if strings.Contains(textLower, "вчера") {
				y := now.AddDate(0, 0, -1)
				startDate, endDate = &y, &y
			} else if strings.Contains(textLower, "сегодня") {
				startDate, endDate = &now, &now
			} else if strings.Contains(textLower, "позавчера") {
				y := now.AddDate(0, 0, -2)
				startDate, endDate = &y, &y
			}
		}

		if startDate != nil {
			s := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
			startDate = &s
		}

		if endDate != nil {
			e := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 999999999, endDate.Location())
			endDate = &e
		} else if startDate != nil {
			e := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 23, 59, 59, 999999999, startDate.Location())
			endDate = &e
		}

		filter := domain.ExpenseFilter{
			UserID:    &userID,
			StartDate: startDate,
			EndDate:   endDate,
		}

		if action.Category != "" {
			filter.Category = &action.Category
		}

		if action.Comment != "" {
			filter.Comment = &action.Comment
		}

		target, err := b.service.FindExpense(ctx, filter)
		if err != nil || target == nil {
			return c.Send("❌ Не удалось найти подходящую запись для изменения.")
		}

		// Prepare Pending Action
		token := fmt.Sprintf("%d", time.Now().UnixNano())

		var commentPtr *string
		if action.NewComment != "" {
			commentPtr = &action.NewComment
		} else {
			commentPtr = target.Comment
		}

		newAmount := target.Amount
		if action.NewAmount > 0 {
			newAmount = action.NewAmount
		}

		newCategory := target.Category
		if action.NewCategory != "" {
			newCategory = action.NewCategory
		}

		newSpentAt := target.SpentAt
		if action.NewDateStr != "" {
			if t, err := time.Parse("2006-01-02", action.NewDateStr); err == nil {
				newSpentAt = t
			}
		}

		actionType := ActionUpdate
		if action.Action == "delete" {
			actionType = ActionDelete
		}

		b.stateManager.Set(token, PendingAction{
			Type:        actionType,
			ExpenseID:   target.ID,
			NewAmount:   newAmount,
			NewCategory: newCategory,
			NewSpentAt:  newSpentAt,
			NewComment:  commentPtr,
		})

		menu := &tele.ReplyMarkup{}
		btnConfirm := menu.Data("✅ Подтвердить", "confirm_action", token)
		btnCancel := menu.Data("❌ Отмена", "cancel_action", token)
		menu.Inline(menu.Row(btnConfirm, btnCancel))

		previewText := fmt.Sprintf(
			"🔍 **Найдена запись для %s:**\nДата: %s\nСумма: %.2f\nКатегория: %s\nКомментарий: %s",
			map[string]string{"update": "обновления", "delete": "удаления"}[action.Action],
			target.SpentAt.Format("02.01.2006"),
			target.Amount,
			target.Category,
			func() string {
				if target.Comment != nil {
					return *target.Comment
				}
				return "—"
			}(),
		)

		return c.Send(previewText, tele.ModeMarkdown, menu)

	default:
		return c.Send("⚠️ Это действие пока не поддерживается ботом.")
	}
}

func (b *Bot) HandleConfirmButton(c tele.Context) error {
	ctx := context.Background()
	token := c.Data()

	pending, exists := b.stateManager.Get(token)

	if !exists {
		return c.Edit("⏰ Время действия истекло или оно уже было обработано.")
	}

	if pending.Type == ActionDelete {
		if err := b.service.DeleteExpense(ctx, pending.ExpenseID); err != nil {
			return c.Edit("❌ Ошибка при удалении записи.")
		}

		return c.Edit("🗑 Запись успешно удалена.")
	} else if pending.Type == ActionUpdate {
		updated := &domain.Expense{
			ID:       pending.ExpenseID,
			UserID:   c.Sender().ID,
			Amount:   pending.NewAmount,
			Category: pending.NewCategory,
			SpentAt:  pending.NewSpentAt,
			Comment:  pending.NewComment,
		}

		if err := b.service.UpdateExpense(ctx, updated); err != nil {
			return c.Edit("❌ Ошибка при обновлении записи.")
		}

		return c.Edit("✅ Запись успешно обновлена.")
	}
	
	return c.Edit("⚠️ Неизвестное действие.")
}

func (b *Bot) HandleCancelButton(c tele.Context) error {
	token := c.Data()

	_, exists := b.stateManager.Get(token)

	if !exists {
		return c.Edit("⏰ Время действия истекло или оно уже было обработано.")
	}
	
	return c.Edit("❌ Действие отменено.")
}

func (b *Bot) Stop() {
	b.teleBot.Stop()
}
