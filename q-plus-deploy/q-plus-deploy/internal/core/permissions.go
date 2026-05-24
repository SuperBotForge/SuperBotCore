package core

import (
	"strings"
)

type DiscordChannelType uint

// Constants for the different bit offsets of intents
const (
	ChannelQueue   DiscordChannelType = 1 << 0
	ChannelStudent DiscordChannelType = 1 << 1
	ChannelTeacher DiscordChannelType = 1 << 2
)

const (
	ChannelAll = ChannelQueue | ChannelStudent | ChannelTeacher
	ChannelAny = 0
)

func (t DiscordChannelType) String() string {
	if t == ChannelAny {
		return "Any"
	}

	var res strings.Builder
	if t&ChannelQueue != 0 {
		res.WriteString("Queue, ")
	}
	if t&ChannelStudent != 0 {
		res.WriteString("Student, ")
	}
	if t&ChannelTeacher != 0 {
		res.WriteString("Teacher, ")
	}

	return strings.TrimSuffix(res.String(), ", ")
}

type DiscordCommandPermission uint

const (
	PermissionAnyone DiscordCommandPermission = iota
	PermissionStudent
	PermissionExaminer
	PermissionAdmin
)
