package parsing

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type DeadlineParseResult struct {
	Original    string
	ParsedDate  *time.Time
	IsValid     bool
	Error       string
	HumanFormat string
}

func ParseDeadline(input string) DeadlineParseResult {
	input = strings.TrimSpace(input)
	// ↓ НОВОЕ: нормализуем
	input = strings.ToLower(input)
	input = strings.ReplaceAll(input, "через ", "") // "через 3 дня" -> "3 дня"
	input = strings.Trim(input, " .,!?:;")          // сносим пунктуацию по краям
	result := DeadlineParseResult{
		Original: input,
		IsValid:  false,
	}

	// Если "не знаю" - это валидно
	if input == "не знаю" {
		result.IsValid = true
		result.HumanFormat = "не определён"
		return result
	}

	// Пробуем как дату
	if date, err := parseDate(input); err == nil {
		result.ParsedDate = &date
		result.IsValid = true
		result.HumanFormat = date.Format("02.01.2006")
		return result
	}

	// Пробуем относительное время
	if date, err := parseRelativeTime(input); err == nil {
		result.ParsedDate = &date
		result.IsValid = true
		result.HumanFormat = date.Format("02.01.2006")
		return result
	}

	result.Error = "Не удалось распознать дедлайн. Примеры: 15.12.2025, 11.11, 1.5 недели, 3 дня, завтра"
	return result
}

func parseRelativeTime(input string) (time.Time, error) {
	in := strings.ToLower(strings.TrimSpace(input))
	now := time.Now()

	// быстрые кейсы-слова
	switch in {
	case "сегодня":
		return now, nil
	case "завтра":
		return now.AddDate(0, 0, 1), nil
	case "послезавтра":
		return now.AddDate(0, 0, 2), nil
	case "неделя":
		return now.AddDate(0, 0, 7), nil
	}

	// поддержим запятую в десятичных неделях
	in = strings.ReplaceAll(in, ",", ".")

	// Регексы (учитываем «дня/дней/дн»; «недель/нед/week/w»; «day/d»)
	daysRe := regexp.MustCompile(`^(\d+)\s*(?:дней|дня|дн|day|d)$`)
	weeksRe := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(?:недель|недели|неделя|нед|week|w)$`)
	monthsRe := regexp.MustCompile(`^(\d+)\s*(?:месяц(?:ев|а)?|мес|month|m)$`)

	if m := weeksRe.FindStringSubmatch(in); m != nil {
		if w, err := strconv.ParseFloat(m[1], 64); err == nil {
			days := int(math.Round(w * 7))
			return now.AddDate(0, 0, days), nil
		}
	}
	if m := daysRe.FindStringSubmatch(in); m != nil {
		if d, err := strconv.Atoi(m[1]); err == nil {
			return now.AddDate(0, 0, d), nil
		}
	}
	if m := monthsRe.FindStringSubmatch(in); m != nil {
		if mo, err := strconv.Atoi(m[1]); err == nil {
			return now.AddDate(0, mo, 0), nil
		}
	}

	// также примем формы типа "3 день", "2 недели", "1 неделя", "1 день"
	looseDays := regexp.MustCompile(`^(\d+)\s*(?:день|дня|дней)$`)
	if m := looseDays.FindStringSubmatch(in); m != nil {
		if d, err := strconv.Atoi(m[1]); err == nil {
			return now.AddDate(0, 0, d), nil
		}
	}
	looseWeeks := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(?:неделя|недели|недель)$`)
	if m := looseWeeks.FindStringSubmatch(in); m != nil {
		if w, err := strconv.ParseFloat(m[1], 64); err == nil {
			return now.AddDate(0, 0, int(math.Round(w*7))), nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid relative time format")
}

// ===== даты =====

func parseDate(input string) (time.Time, error) {
	input = strings.TrimSpace(input)
	loc := time.Now().Location()

	// поддержка формата dd.mm (год подставляем)
	if m := regexp.MustCompile(`^\s*(\d{1,2})[.\-/](\d{1,2})\s*$`).FindStringSubmatch(input); m != nil {
		day, _ := strconv.Atoi(m[1])
		mon, _ := strconv.Atoi(m[2])
		now := time.Now().In(loc)
		cand := time.Date(now.Year(), time.Month(mon), day, 0, 0, 0, 0, loc)
		// если уже прошла — переносим на следующий год
		if !isSameOrAfter(cand, now) {
			cand = cand.AddDate(1, 0, 0)
		}
		return cand, nil
	}

	// обычные форматы с годом
	formats := []string{
		"02.01.2006",
		"2.1.2006",
		"02.01.06",
		"2.1.06",
		"02/01/2006",
		"2/1/2006",
		"02-01-2006",
		"2-1-2006",
		"2006-01-02",
		"2006/01/02",
	}

	for _, f := range formats {
		if d, err := time.ParseInLocation(f, input, loc); err == nil {
			// считаем валидным, если дата не раньше сегодняшней (по дню)
			now := time.Now().In(loc)
			if isSameOrAfter(d, now) {
				// нормализуем к «началу дня» (или оставь как есть, если нужно время 00:00)
				return today(d), nil
			}
			return time.Time{}, fmt.Errorf("date is in the past")
		}
	}

	return time.Time{}, fmt.Errorf("invalid date format")
}

// ===== утилиты =====

func today(t time.Time) time.Time {
	loc := t.Location()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func isSameOrAfter(a, b time.Time) bool {
	aa := today(a)
	bb := today(b)
	return !aa.Before(bb)
}
