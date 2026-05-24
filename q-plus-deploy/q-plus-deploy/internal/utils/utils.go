package utils

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
	"q+/internal/generated/ent"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Provider[T any] interface {
	Get() T
}
type provider[T any] struct {
	provide func() T
}

func (p *provider[T]) Get() T {
	return p.provide()
}

func MakeProvider[T any](provide func() T) Provider[T] {
	return &provider[T]{provide: provide}
}

func LimitString(str string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if utf8.RuneCountInString(str) > maxLen {
		if maxLen <= 3 {
			return string([]rune(str)[:maxLen])
		} else {
			return string([]rune(str)[:maxLen-2]) + ".."
		}
	}
	return str
}

func CatchPanicAsError[T any](f func() T) (result T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()
	result = f()
	return
}

func Prepend[T any](slice []T, elems ...T) []T {
	return append(elems, slice...)
}

func CreateGoogleSheetsSheetLink(spreadsheetId string, sheetId int64) string {
	return "https://docs.google.com/spreadsheets/d/" + spreadsheetId + "/edit#gid=" + strconv.FormatInt(sheetId, 10)
}

func CreateGoogleSheetsLink(spreadsheetId string) string {
	return "https://docs.google.com/spreadsheets/d/" + spreadsheetId + "/edit"
}

var linkRE = regexp.MustCompile(`.*/d/(.*)/.*gid=(\d+)`)

func ParseGoogleSheetsLink(link string) (spreadsheetId string, sheetId int64, err error) {
	matches := linkRE.FindStringSubmatch(link)

	if matches == nil || len(matches) != 3 || len(matches[1]) == 0 {
		return "", -1, errors.New("parse failed")
	}

	spreadsheetId = matches[1]
	sheetId, err = strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return "", -1, err
	}

	return
}

var parseDurationRE = regexp.MustCompile(`^(\d+d)?(\d+h)?(\d+m)?$`)

// ParseDuration parses a duration string in the format "1d2h30m", "05m", "0" etc.
func ParseDuration(durationStr string) (time.Duration, error) {
	if durationStr == "0" {
		return 0, nil
	}

	matches := parseDurationRE.FindStringSubmatch(durationStr)

	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid duration format")
	}

	var days, hours, minutes int

	for _, match := range matches[1:] {
		if match == "" {
			continue
		}

		unit := match[len(match)-1]
		value, err := strconv.Atoi(strings.TrimSuffix(match, string(unit)))
		if err != nil {
			return 0, err
		}

		switch unit {
		case 'd':
			days = value
		case 'h':
			hours = value
		case 'm':
			minutes = value
		default:
			return 0, fmt.Errorf("invalid time unit")
		}
	}

	totalDuration := time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute

	return totalDuration, nil
}

// PrintDuration serializes a duration string in the format "1d2h30m", "05m", "0" etc.
func PrintDuration(duration time.Duration) string {
	if duration == 0 {
		return "0"
	}

	days := duration / (24 * time.Hour)
	hours := (duration % (24 * time.Hour)) / time.Hour
	minutes := (duration % time.Hour) / time.Minute

	var result strings.Builder

	if days > 0 {
		result.WriteString(strconv.Itoa(int(days)))
		result.WriteString("d")
	}
	if hours > 0 {
		result.WriteString(strconv.Itoa(int(hours)))
		result.WriteString("h")
	}
	if minutes > 0 {
		result.WriteString(strconv.Itoa(int(minutes)))
		result.WriteString("m")
	}

	return result.String()
}

func FormatNilTime(t *time.Time, layout string) string {
	if t == nil {
		return "-"
	}
	return t.Format(layout)
}

func JoinCriteria(criteria []*ent.Criterion) string {
	var criteriaList strings.Builder
	if len(criteria) > 0 {
		for _, criterion := range criteria[:len(criteria)-1] {
			criteriaList.WriteString(criterion.Name)
			criteriaList.WriteString(", ")
		}
		criteriaList.WriteString(criteria[len(criteria)-1].Name)
	}
	return criteriaList.String()
}

func JoinUserPings(users []*ent.User) string {
	return strings.Join(lo.Map(users, func(u *ent.User, _ int) string { return "<@" + u.DiscordID + ">" }), ", ")
}

func MentionU(userDiscordId string) string {
	u := discordgo.User{ID: userDiscordId}
	return u.Mention()
}
