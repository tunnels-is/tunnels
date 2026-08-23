package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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

	link := func(name, hint, href string) fyne.CanvasObject {
		return container.NewVBox(
			ghostBtn(name, func() { a.openURL(href) }),
			caption(hint),
			vspace(8),
		)
	}

	return pageScroll(
		caption(fmt.Sprintf("App %s  ·  API %s", ver, api)),
		card("Need a hand?", "Start with the documentation, then reach out on Discord or email if you're stuck.", container.NewHBox(
			primaryBtn("Read docs", func() { a.openURL("https://www.tunnels.is/docs") }),
			ghostBtn("Discord", func() { a.openURL("https://discord.gg/2v5zX5cG3j") }),
		)),
		card("Resources", "Website, documentation and source code.", container.NewVBox(
			link("Website", "tunnels.is", "https://tunnels.is"),
			link("Documentation", "tunnels.is/docs", "https://www.tunnels.is/docs"),
			link("GitHub", "tunnels-is/tunnels", "https://www.github.com/tunnels-is/tunnels"),
		)),
		card("Direct contact", "Email is the fastest way to reach us about billing or security.", container.NewVBox(
			link("Email", "support@tunnels.is", "mailto:support@tunnels.is"),
		)),
		card("Community", "Join the public chat rooms.", container.NewVBox(
			link("Discord", "discord.gg/2v5zX5cG3j", "https://discord.gg/2v5zX5cG3j"),
			link("X", "x.com/tunnels_is", "https://www.x.com/tunnels_is"),
			link("Reddit", "reddit.com/r/tunnels_is", "https://www.reddit.com/r/tunnels_is"),
			link("Signal", "signal.group", "https://signal.group/#CjQKIGvNLjUd8o3tkkGUZHuh0gfZqHEsn6rxXOG4S1U7m2lEEhBtuWbyxBjMLM_lo1rVjFX0"),
		)),
	)
}
