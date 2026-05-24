package handlers

import (
	"github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
	"q+/internal/utils"
	"strings"
	"time"
)

// TODO options as typed struct for every command??
/*
	maybe really create struct for every command
	and create functions for different option types
	so it will looks like:
	type NextOptions struct {
		someName string
		someNumber *int
	}
	NextOptions{
		someName: options.String("someName"),
		someNumber: options.OptInt("someNumber"),
	}
	and this functions will panic if option not found or not correct type
	and some recovery function will send error message to user

	this will be server side validation, like I wanted
*/
type OptionMap map[string]*discordgo.ApplicationCommandInteractionDataOption // TODO convert to interface

func (o OptionMap) get(name string) *discordgo.ApplicationCommandInteractionDataOption {
	if v, ok := o[name]; ok {
		return v
	}
	panic("option '" + name + "' not found")
}

func (o OptionMap) String(name string) string {
	return o.get(name).StringValue()
}

func (o OptionMap) OptString(name string) *string {
	if v, ok := o[name]; ok {
		return lo.ToPtr(v.StringValue())
	}
	return nil
}

func (o OptionMap) Int(name string) int64 {
	return o.get(name).IntValue()
}

func (o OptionMap) OptInt(name string) *int64 {
	if v, ok := o[name]; ok {
		return lo.ToPtr(v.IntValue())
	}
	return nil
}

func (o OptionMap) Uint(name string) uint64 {
	return o.get(name).UintValue()
}

func (o OptionMap) OptUint(name string) *uint64 {
	if v, ok := o[name]; ok {
		return lo.ToPtr(v.UintValue())
	}
	return nil
}

func (o OptionMap) Bool(name string) bool {
	return o.get(name).BoolValue()
}

func (o OptionMap) OptBool(name string) *bool {
	if v, ok := o[name]; ok {
		return lo.ToPtr(v.BoolValue())
	}
	return nil

}

func (o OptionMap) Channel(name string, s *discordgo.Session) *discordgo.Channel {
	return o.get(name).ChannelValue(s)
}

func (o OptionMap) OptChannel(name string, s *discordgo.Session) *discordgo.Channel {
	if v, ok := o[name]; ok {
		return v.ChannelValue(s)
	}
	return nil
}

func (o OptionMap) ChannelId(name string) string {
	return o.get(name).ChannelValue(nil).ID
}

func (o OptionMap) OptChannelId(name string) *string {
	if v, ok := o[name]; ok {
		return lo.ToPtr(v.ChannelValue(nil).ID)
	}
	return nil
}

func (o OptionMap) User(name string, ctx InteractionContext) *discordgo.Member {
	user := o.get(name).UserValue(ctx.S)
	member, err := ctx.S.GuildMember(ctx.I.GuildID, user.ID)
	if err != nil {
		panic(err)
	}
	return member
}

func (o OptionMap) OptUser(name string, ctx InteractionContext) *discordgo.Member {
	if v, ok := o[name]; ok {
		user := v.UserValue(ctx.S)
		member, err := ctx.S.GuildMember(ctx.I.GuildID, user.ID)
		if err != nil {
			panic(err)
		}
		return member
	}
	return nil
}

func (o OptionMap) RoleId(name string) string {
	return o.get(name).RoleValue(nil, "").ID
}

func (o OptionMap) Duration(name string) time.Duration {
	value := o.String(name)
	duration, err := utils.ParseDuration(value)
	if err != nil {
		panic(err)
	}
	return duration
}

func (o OptionMap) OptDuration(name string) *time.Duration {
	if v, ok := o[name]; ok {
		value := v.StringValue()
		duration, err := utils.ParseDuration(value)
		if err != nil {
			panic(err)
		}
		return &duration
	}
	return nil
}

func (o OptionMap) Time(name string) time.Time {
	value := o.String(name)
	loc, err := time.LoadLocation("Asia/Novosibirsk")
	if err != nil {
		panic(err)
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", value, loc)
	if err != nil {
		panic(err)
	}
	return t
}

func (o OptionMap) OptTime(name string) *time.Time {
	if v, ok := o[name]; ok {
		value := strings.TrimSpace(v.StringValue())
		if value == "-" {
			t := time.Time{}
			return &t
		}
		loc, err := time.LoadLocation("Asia/Novosibirsk")
		if err != nil {
			panic(err)
		}
		t, err := time.ParseInLocation("2006-01-02 15:04", value, loc)
		if err != nil {
			panic(err)
		}
		return &t
	}
	return nil
}
