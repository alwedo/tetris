package client

import "charm.land/lipgloss/v2"

var helpStyle = lipgloss.NewStyle().
	Align(lipgloss.Center).
	Border(lipgloss.NormalBorder(), true, false, false).
	Foreground(lipgloss.Color("#FF75B7"))

var faintStyle = lipgloss.NewStyle().Faint(true)

var boldStyle = lipgloss.NewStyle().Bold(true)
