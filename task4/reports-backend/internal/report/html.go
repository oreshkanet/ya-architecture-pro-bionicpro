package report

import (
	"fmt"
	"html"
	"strings"

	"reports-backend/internal/repository"
)

func RenderHTML(r *repository.ReportRow) string {
	regAt := ""
	if r.RegisteredAt != nil {
		regAt = r.RegisteredAt.Format("02.01.2006")
	}
	sb := &strings.Builder{}
	sb.WriteString("<!DOCTYPE html>\n<html lang=\"ru\">\n<head>\n<meta charset=\"UTF-8\">\n")
	sb.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	sb.WriteString("<title>Отчёт по использованию</title>\n")
	sb.WriteString("<style>body{font-family:system-ui,sans-serif;max-width:800px;margin:2rem auto;padding:0 1rem;} table{border-collapse:collapse;width:100%;} th,td{border:1px solid #ddd;padding:0.5rem 1rem;text-align:left;} th{background:#f5f5f5;}</style>\n")
	sb.WriteString("</head>\n<body>\n")
	sb.WriteString("<h1>Отчёт по использованию</h1>\n")
	sb.WriteString("<p><strong>Пользователь:</strong> " + html.EscapeString(r.FullName) + " (" + html.EscapeString(r.Email) + ")</p>\n")
	if regAt != "" {
		sb.WriteString("<p><strong>Дата регистрации:</strong> " + html.EscapeString(regAt) + "</p>\n")
	}
	sb.WriteString("<table>\n")
	sb.WriteString("<tr><th>Показатель</th><th>Значение</th></tr>\n")
	sb.WriteString("<tr><td>Период</td><td>" + html.EscapeString(r.PeriodStart) + " — " + html.EscapeString(r.PeriodEnd) + "</td></tr>\n")
	sb.WriteString("<tr><td>Часы использования</td><td>" + fmt.Sprintf("%.2f", r.UsageHours) + "</td></tr>\n")
	sb.WriteString("<tr><td>Количество сессий</td><td>" + fmt.Sprintf("%d", r.SessionCount) + "</td></tr>\n")
	sb.WriteString("<tr><td>Среднее в день (ч)</td><td>" + fmt.Sprintf("%.2f", r.AvgDailyUse) + "</td></tr>\n")
	sb.WriteString("<tr><td>Обновлено</td><td>" + html.EscapeString(r.UpdatedAt.Format("02.01.2006 15:04")) + "</td></tr>\n")
	sb.WriteString("</table>\n</body>\n</html>")
	return sb.String()
}
