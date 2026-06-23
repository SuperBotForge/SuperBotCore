package model

type GlobalUserID int64

type PlatformUserID string

type GlobalUser struct {
	ID             GlobalUserID     `json:"id"`
	TsuAccountsID  *string          `json:"tsu_accounts_id,omitempty"`
	PrimaryChannel ChannelType      `json:"primary_channel"`
	ProfileData    map[string]any   `json:"profile_data,omitempty"`
	Locale         string           `json:"locale"`
	Role           string           `json:"role"`
	Accounts       []ChannelAccount `json:"accounts,omitempty"`
}

func (u *GlobalUser) PlatformUserID() PlatformUserID {
	for _, acc := range u.Accounts {
		if acc.ChannelType == u.PrimaryChannel {
			return acc.ChannelUserID
		}
	}
	return ""
}

// UserInfo contains basic information about a user, returned to WASM plugins.
type UserInfo struct {
	ID         int64  `json:"id"`
	FullName   string `json:"full_name,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
	IsTeacher  bool   `json:"is_teacher,omitempty"`
}
