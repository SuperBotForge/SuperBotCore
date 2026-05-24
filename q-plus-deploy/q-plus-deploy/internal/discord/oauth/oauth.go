package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/samber/do/v2"
	"strconv"
	"time"
)

type Config struct {
	ClientID     string
	ClientSecret string
	BackendUrl   string
}

type StateData struct {
	CreatedAt     time.Time
	AppId         string
	GuildId       string
	StudentRoles  []discordgo.Role
	ExaminerRoles []discordgo.Role
}

type Oauth struct {
	Config Config
	States map[string]*StateData
}

func NewOauth(i do.Injector) (*Oauth, error) {
	config := do.MustInvoke[Config](i)
	return &Oauth{
		Config: config,
		States: make(map[string]*StateData),
	}, nil
}

func (c *Oauth) RedirectPath() string {
	return "/d-auth"
}

func (c *Oauth) RedirectUrl() string {
	return c.Config.BackendUrl + c.RedirectPath()
}

func (c *Oauth) AuthUrl(stateData *StateData) string {
	// create a random 32 byte state and base64 encode it
	var buffer = make([]byte, 32)
	_, err := rand.Read(buffer)
	state := base64.URLEncoding.EncodeToString(buffer)
	if err != nil {
		state = strconv.FormatInt(time.Now().Unix(), 10)
	}

	c.States[state] = stateData

	authUrl := fmt.Sprintf(
		"%s?response_type=code&client_id=%s&scope=identify+applications.commands.permissions.update&state=%s&redirect_uri=%s&prompt=none&integration_type=0",
		discordgo.EndpointOAuth2Authorize,
		c.Config.ClientID,
		state,
		c.RedirectUrl(),
	)
	return authUrl
}
