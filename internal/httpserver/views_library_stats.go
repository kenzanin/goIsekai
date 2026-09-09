package httpserver

import (
	"fmt"
	"goisekai/internal/database"
)

// buildOverviewStrings creates the display strings for the library overview
// stats section: statusLine, readLine, readingTime, mostLine, fewestLine.
func buildOverviewStrings(overview database.LibraryOverview) (statusLine, readLine, readingTime, mostLine, fewestLine string) {
	statusParts := make([]string, 0, 3)
	if overview.StatusDone > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d done", overview.StatusDone))
	}
	if overview.StatusOngoing > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d ongoing", overview.StatusOngoing))
	}
	if overview.StatusUnknown > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d unknown", overview.StatusUnknown))
	}
	statusLine = joinStatusParts(statusParts)

	readLine = fmt.Sprintf("%d finished · %d reading", overview.FullyRead, overview.StartedReading)
	readingTime = fmt.Sprintf("%.1f h", float64(overview.PagesRead)*120/3600)

	mostLine = formatMostTitle(overview.MostTitle, overview.MostCount, overview.MostDup)
	fewestLine = formatFewestTitle(overview.FewestTitle, overview.FewestCount, overview.FewestDup)
	return
}

func joinStatusParts(parts []string) string {
	if len(parts) == 0 {
		return "no data"
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += " · " + p
	}
	return result
}

func formatMostTitle(title string, count, dup int) string {
	short := fmt.Sprintf("%d ch", count)
	if dup > 1 {
		return fmt.Sprintf("%d ch · %d titles", count, dup)
	}
	if len(title) > 25 {
		return fmt.Sprintf("%s… · %d ch", title[:25], count)
	}
	if title != "" {
		return fmt.Sprintf("%s · %d ch", title, count)
	}
	return short
}

func formatFewestTitle(title string, count, dup int) string {
	short := fmt.Sprintf("%d ch", count)
	if dup > 1 {
		return fmt.Sprintf("%d ch · %d titles", count, dup)
	}
	if len(title) > 25 {
		return fmt.Sprintf("%s… · %d ch", title[:25], count)
	}
	if title != "" {
		return fmt.Sprintf("%s · %d ch", title, count)
	}
	return short
}
