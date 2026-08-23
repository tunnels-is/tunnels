package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"github.com/tunnels-is/tunnels/version"
)

func (a *App) supportPage() fyne.CanvasObject {
	ver := version.Version
	api := fmt.Sprintf("%d", version.ApiVersion)
	if a.state != nil {
		if a.state.Version != "" {
			ver = a.state.Version
		}
		if a.state.APIVersion != 0 {
			api = fmt.Sprintf("%d", a.state.APIVersion)
		}
	}

	quickStart := card("Need a hand?",
		"Start with the documentation. If that does not cover it, Discord is the fastest way to reach us.",
		hstack(sp2,
			primaryBtn("Read the docs", func() { a.openURL("https://www.tunnels.is/docs") }),
			outlineBtn("Join Discord", func() { a.openURL("https://discord.gg/2v5zX5cG3j") }),
		))

	resources := card("Resources", "", settingList(
		linkRow(a, "Website", "tunnels.is", "https://tunnels.is"),
		linkRow(a, "Documentation", "tunnels.is/docs", "https://www.tunnels.is/docs"),
		linkRow(a, "Source code", "github.com/tunnels-is/tunnels", "https://www.github.com/tunnels-is/tunnels"),
	))

	contact := card("Direct contact",
		"Email is the fastest route for billing and security questions.",
		settingList(
			linkRow(a, "Email support", "support@tunnels.is", "mailto:support@tunnels.is"),
		))

	community := card("Community", "", settingList(
		linkRow(a, "Discord", "discord.gg/2v5zX5cG3j", "https://discord.gg/2v5zX5cG3j"),
		linkRow(a, "X", "x.com/tunnels_is", "https://www.x.com/tunnels_is"),
		linkRow(a, "Reddit", "reddit.com/r/tunnels_is", "https://www.reddit.com/r/tunnels_is"),
		linkRow(a, "Signal", "signal.group", "https://signal.group/#CjQKIGvNLjUd8o3tkkGUZHuh0gfZqHEsn6rxXOG4S1U7m2lEEhBtuWbyxBjMLM_lo1rVjFX0"),
	))

	about := card("About", "", vstack(0,
		kvRow("App version", ver, true),
		kvRow("API version", api, true),
	))

	sub := fmt.Sprintf("Tunnels %s · API v%s", ver, api)
	return pageShell("Support", sub, nil, scrollBody(quickStart, resources, contact, community, about))
}
