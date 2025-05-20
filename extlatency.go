package extlatency

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func Parse(logStr string) any {
	datapowerLogRegex := regexp.MustCompile(`(?:ExtLatency: )(.*)(?: == )(.*)(\[.*\])$`) // must try first
	apiGatewayLogRegex := regexp.MustCompile(`(?:ExtLatency: )(.*)(\[.*\])$`)

	if datapowerLogRegex.MatchString(logStr) {
		fmt.Println("Parse as DP Log")
	} else if apiGatewayLogRegex.MatchString(logStr) {
		fmt.Println("Handle as APIC Log")
		match := apiGatewayLogRegex.FindStringSubmatch(logStr)
		if len(match) > 0 {
			actionsRaw := strings.Trim(match[1], ", ")
			actionsRawSplit := strings.Split(actionsRaw, ",")
			rawActions := parseActionsBase(actionsRawSplit)
			// fmt.Println(rawActions)
			actions := parseActions(rawActions)
			// fmt.Println(actions)
			baseNode, err := nestActionsByTransaction(actions)
			if err != nil {
				log.Fatalln(err)
			}
			fmt.Println(baseNode.name)
			fmt.Println(baseNode.duration)
			fmt.Println(baseNode.children)
		}
	} else {
		fmt.Println("Handle as APIC Log")
	}
	return logStr
}

func getDescriptionMap() map[string]string {
	filePath := "descriptions.json"
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}
	var data map[string]string
	err = json.Unmarshal(fileContent, &data)
	if err != nil {
		log.Fatalf("Error unmarshaling JSON: %v", err)
	}
	// fmt.Println("Parsed JSON data:", data)
	return data
}

type BaseAction struct {
	keyword string
	elapsed int
}

type Action struct {
	BaseAction
	description string
	duration    int
}

type Node struct {
	name     string
	duration int
	children []Action
}

func parseActions(baseActions []BaseAction) []Action {
	var actions []Action
	descMap := getDescriptionMap()
	for i, baseAction := range baseActions {
		duration := 0
		if i != 0 {
			duration = baseAction.elapsed - baseActions[i-1].elapsed
		}
		action := Action{
			BaseAction:  baseAction,
			description: descMap[baseAction.keyword],
			duration:    duration,
		}
		actions = append(actions, action)
	}
	return actions
}

func parseActionsBase(actionsRawSplit []string) []BaseAction {
	var actions []BaseAction
	for _, actionStrRaw := range actionsRawSplit {
		splitStrs := strings.Split(actionStrRaw, "=")
		keyword := splitStrs[0]
		elapsed, err := strconv.ParseInt(splitStrs[1], 0, 64)
		if err != nil {
			log.Fatalf("Failed to parse action %s elapsed time", keyword)
		}
		action := BaseAction{keyword, int(elapsed)}
		actions = append(actions, action)
	}
	return actions
}

func nestActionsByTransaction(actions []Action) (Node, error) {
	firstAction := actions[0]
	lastAction := actions[len(actions)-1]
	if firstAction.keyword == "TS" && lastAction.keyword == "TC" {
		children := actions[1 : len(actions)-1]
		for i, value := range children {
			if value.keyword == "TS" || value.keyword == "TC" {
				log.Fatalf("Log contains more than one transaction, %s found on index %d", value.keyword, i)
			}
		}
		return Node{
			name:     "Transaction",
			duration: lastAction.elapsed,
			children: children,
		}, nil
	}
	return Node{}, errors.New("log does not start and end with TS and TC respectively")
}
