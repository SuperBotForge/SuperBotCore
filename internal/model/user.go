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

// UserPosition holds one student or teacher position for a user.
type UserPosition struct {
	PositionType    string `json:"position_type"`
	Status          string `json:"status,omitempty"`
	NationalityType string `json:"nationality_type,omitempty"`
	FundingType     string `json:"funding_type,omitempty"`
	EducationForm   string `json:"education_form,omitempty"`
	GroupCode       string `json:"group_code,omitempty"`
	GroupName       string `json:"group_name,omitempty"`
	ProgramName     string `json:"program_name,omitempty"`
	StreamName      string `json:"stream_name,omitempty"`
}

// UserInfoFull extends UserInfo with university positions.
type UserInfoFull struct {
	UserInfo
	Positions []UserPosition `json:"positions,omitempty"`
}
