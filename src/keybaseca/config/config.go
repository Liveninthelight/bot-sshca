package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/keybase/bot-ssh-ca/src/keybaseca/botwrapper"

	"github.com/keybase/bot-ssh-ca/src/shared"
)

// Represents a loaded and validated config for keybaseca
type Config interface {
	GetCAKeyLocation() string
	GetKeybaseHomeDir() string
	GetKeybasePaperKey() string
	GetKeybaseUsername() string
	GetKeyExpiration() string
	GetTeams() []string
	GetChatTeam() string
	GetChannelName() string
	GetLogLocation() string
	GetStrictLogging() bool
}

func ValidateConfig(conf EnvConfig) error {
	if len(conf.GetTeams()) == 0 {
		return fmt.Errorf("must specify at least one team via the TEAMS environment variable")
	}
	if conf.GetKeyExpiration() != "" && !strings.HasPrefix(conf.GetKeyExpiration(), "+") {
		// Only a basic check for this since ssh will error out later on if it is bogus
		return fmt.Errorf("KEY_EXPIRATION must be of the form `+<number><unit> where unit is one of `m`, `h`, `d`, `w`. Eg `+1h`. ")
	}
	if conf.GetLogLocation() != "" {
		err := validatePath(conf.GetLogLocation())
		if err != nil {
			return fmt.Errorf("LOG_LOCATION '%s' is not a valid path: %v", conf.GetLogLocation(), err)
		}
	}
	if conf.getChatChannel() != "" {
		team, channel, err := splitTeamChannel(conf.getChatChannel())
		if err != nil {
			return fmt.Errorf("Failed to parse CHAT_CHANNEL=%s: %v", conf.getChatChannel(), err)
		}
		err = validateChannel(&conf, team, channel)
		if err != nil {
			return fmt.Errorf("failed to validate CHAT_CHANNEL '%s': %v", channel, err)
		}
	}
	if conf.getStrictLogging() != "" {
		if conf.getStrictLogging() != "true" && conf.getStrictLogging() != "false" {
			return fmt.Errorf("STRICT_LOGGING must be either 'true' or 'false', '%s' is not valid", conf.getStrictLogging())
		}
	}
	return nil
}

// Validates the given teamName and channelName to determine whether or not the given channelName is the name
// of a channel inside the given team. Returns nil if everything validates.
func validateChannel(conf Config, teamName string, channelName string) error {
	api, err := botwrapper.GetKBChat(conf.GetKeybaseHomeDir(), conf.GetKeybasePaperKey(), conf.GetKeybaseUsername())
	if err != nil {
		return err
	}
	result, err := api.ListChannels(teamName)
	if err != nil {
		return err
	}

	for _, channel := range result {
		if channel == channelName {
			// The channel does exist, but the bot may or may not be in it. So join the channel in order to ensure
			// the bot will receive chat events from it
			_, err := api.JoinChannel(teamName, channelName)
			if err != nil {
				return fmt.Errorf("failed to join bot to the configured channel: %v", err)
			}
			return nil
		}
	}
	return fmt.Errorf("did not find a channel named %s in %s", channelName, teamName)
}

// Returns an error if the given path is not a writable path on the local filesystem or on KBFS
func validatePath(path string) error {
	if strings.HasPrefix(path, "/keybase/") {
		// If it exists it is valid
		exists, _ := shared.KBFSFileExists(path)
		if exists {
			return nil
		}

		// Otherwise try to write to it
		err := shared.KBFSWrite(path, "", false)
		if err != nil {
			return fmt.Errorf("path is not writable: %v", err)
		}
		shared.KBFSDelete(path)
		return nil
	}
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}

	var d []byte
	err = ioutil.WriteFile(path, d, 0600)
	if err != nil {
		return fmt.Errorf("path is not writable: %v", err)
	}
	os.Remove(path)
	return nil
}

type EnvConfig struct{}

var _ Config = (*EnvConfig)(nil)

func (ef *EnvConfig) GetCAKeyLocation() string {
	if os.Getenv("CA_KEY_LOCATION") != "" {
		return shared.ExpandPathWithTilde(os.Getenv("CA_KEY_LOCATION"))
	}
	return shared.ExpandPathWithTilde("/mnt/keybase-ca-key")
}

func (ef *EnvConfig) GetKeybaseHomeDir() string {
	return os.Getenv("KEYBASE_HOME_DIR")
}

func (ef *EnvConfig) GetKeybasePaperKey() string {
	return os.Getenv("KEYBASE_PAPERKEY")
}

func (ef *EnvConfig) GetKeybaseUsername() string {
	return os.Getenv("KEYBASE_USERNAME")
}

func (ef *EnvConfig) GetKeyExpiration() string {
	if os.Getenv("KEY_EXPIRATION") != "" {
		return os.Getenv("KEY_EXPIRATION")
	}
	return "+1h"
}

func (ef *EnvConfig) GetTeams() []string {
	split := strings.Split(os.Getenv("TEAMS"), ",")
	var teams []string
	for _, item := range split {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			teams = append(teams, trimmed)
		}
	}
	return teams
}

func (ef *EnvConfig) GetLogLocation() string {
	return os.Getenv("LOG_LOCATION")
}

func (ef *EnvConfig) getStrictLogging() string {
	return strings.ToLower(os.Getenv("STRICT_LOGGING"))
}

func (ef *EnvConfig) GetStrictLogging() bool {
	return ef.getStrictLogging() == "true"
}

func (ef *EnvConfig) getChatChannel() string {
	return os.Getenv("CHAT_CHANNEL")
}

func (ef *EnvConfig) GetChatTeam() string {
	if ef.getChatChannel() == "" {
		return ""
	}
	team, _, err := splitTeamChannel(ef.getChatChannel())
	if err != nil {
		panic("Failed to retrieve chat team! This should never happen due to config validation...")
	}
	return team
}

func (ef *EnvConfig) GetChannelName() string {
	if ef.getChatChannel() == "" {
		return ""
	}
	_, channel, err := splitTeamChannel(ef.getChatChannel())
	if err != nil {
		panic("Failed to retrieve channel name! This should never happen due to config validation...")
	}
	return channel
}

// Split a teamChannel of the form team.foo.bar#chan into "team.foo.bar", "chan"
func splitTeamChannel(teamChannel string) (string, string, error) {
	split := strings.Split(teamChannel, "#")
	if len(split) != 2 {
		return "", "", fmt.Errorf("'%s' is not a valid specifier for a team and a channel", teamChannel)
	}
	return split[0], split[1], nil
}