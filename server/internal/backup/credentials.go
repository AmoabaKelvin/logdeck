package backup

import (
	"slices"

	"github.com/AmoabaKelvin/logdeck/internal/config"
)

// redactCredentials strips every secret from a config file: the sign-in
// settings and API tokens entirely, platform API tokens, and alert channel
// URLs and tokens (a webhook or ntfy URL is itself the credential).
func redactCredentials(fc config.FileConfig) config.FileConfig {
	fc.Auth = nil
	fc.APITokens = nil
	fc.CoolifyHosts = slices.Clone(fc.CoolifyHosts)
	for i := range fc.CoolifyHosts {
		fc.CoolifyHosts[i].APIToken = ""
	}
	fc.DokployHosts = slices.Clone(fc.DokployHosts)
	for i := range fc.DokployHosts {
		fc.DokployHosts[i].APIToken = ""
	}
	if fc.Alerts != nil {
		alerts := *fc.Alerts
		alerts.WebhookURL = ""
		alerts.Channels = slices.Clone(alerts.Channels)
		for i := range alerts.Channels {
			alerts.Channels[i].URL = ""
			alerts.Channels[i].Token = ""
		}
		fc.Alerts = &alerts
	}
	return fc
}

// keepCredentials fills a config restored from a backup made without
// credentials from the running instance. Sign-in and API tokens stay exactly
// as they are, so restoring never changes who can get in; a platform host or
// alert channel takes its secret from the current one with the same name and
// URL, or the same ID, so a token never goes to an address it was not issued
// for. A channel left without one is disabled rather than failing every
// delivery. It returns how many still need their secret entered again.
func keepCredentials(restored *config.FileConfig, current config.FileConfig) int {
	restored.Auth = current.Auth
	restored.APITokens = current.APITokens
	missing := 0

	for i, h := range restored.CoolifyHosts {
		if h.APIToken != "" {
			continue
		}
		if j := slices.IndexFunc(current.CoolifyHosts, func(c config.CoolifyHostConfig) bool { return c.HostName == h.HostName && c.APIURL == h.APIURL }); j >= 0 {
			restored.CoolifyHosts[i].APIToken = current.CoolifyHosts[j].APIToken
		} else {
			missing++
		}
	}
	for i, h := range restored.DokployHosts {
		if h.APIToken != "" {
			continue
		}
		if j := slices.IndexFunc(current.DokployHosts, func(c config.DokployHostConfig) bool { return c.HostName == h.HostName && c.APIURL == h.APIURL }); j >= 0 {
			restored.DokployHosts[i].APIToken = current.DokployHosts[j].APIToken
		} else {
			missing++
		}
	}

	if restored.Alerts == nil {
		return missing
	}
	var currentChannels []config.AlertChannel
	if current.Alerts != nil {
		currentChannels = current.Alerts.Channels
	}
	for i, ch := range restored.Alerts.Channels {
		if ch.URL != "" || ch.Token != "" {
			continue
		}
		if j := slices.IndexFunc(currentChannels, func(c config.AlertChannel) bool { return c.ID == ch.ID && c.Type == ch.Type }); j >= 0 {
			restored.Alerts.Channels[i].URL = currentChannels[j].URL
			restored.Alerts.Channels[i].Token = currentChannels[j].Token
		} else {
			restored.Alerts.Channels[i].Enabled = false
			missing++
		}
	}
	return missing
}
