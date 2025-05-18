package extlatency

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func Parse(log string) any {
	datapowerLogRegex := regexp.MustCompile(`(?:ExtLatency: )(.*)(?: == )(.*)(\[.*\])$`) // must try first
	apiGatewayLogRegex := regexp.MustCompile(`(?:ExtLatency: )(.*)(\[.*\])$`)

	if datapowerLogRegex.MatchString(log) {
		fmt.Println("Parse as DP Log")
	} else if apiGatewayLogRegex.MatchString(log) {
		fmt.Println("Handle as APIC Log")
		match := apiGatewayLogRegex.FindStringSubmatch(log)
		if len(match) > 0 {
			// fmt.Println(match[0])
			actionsRaw := strings.Split(match[1], ",")
			// fmt.Println(actionsRaw)

			parseActions(actionsRaw)
			// logUrl := match[2]
			// logUrl = logUrl[1: len(logUrl)-1]
			// fmt.Println(logUrl)

		}
	} else {
		fmt.Println("Handle as APIC Log")
	}

	return log
}

type Action struct {
	keyword string
	start   int64
	// duration    int
	description string
}

func NewAction(keyword string, start int64) Action {
	return Action{
		keyword:     keyword,
		start:       start,
		description: "",
	}
}
func parseActions(actionsRaw []string) {
	var actionsParsed []Action
	for _, actionRaw := range actionsRaw {
		if strings.TrimSpace(actionRaw) != "" {
			// fmt.Println("Action" + actionRaw)
			splitStrs := strings.Split(actionRaw, "=")
			keyword := splitStrs[0]
			start, _ := strconv.ParseInt(splitStrs[1], 0, 64)
			actionsParsed = append(actionsParsed, NewAction(keyword, start))
		}
	}
	fmt.Println(actionsParsed)

}
