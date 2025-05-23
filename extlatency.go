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
		// fmt.Println("Parse as DP Log")
	} else if apiGatewayLogRegex.MatchString(logStr) {
		// fmt.Println("Handle as APIC Log")
		match := apiGatewayLogRegex.FindStringSubmatch(logStr)
		if len(match) > 0 {
			actionsRaw := strings.Trim(match[1], ", ")
			actionsRawSplit := strings.Split(actionsRaw, ",")
			rawActions := parseActionsBase(actionsRawSplit)
			// fmt.Println(rawActions)
			actions := parseActions(rawActions)
			// fmt.Println(actions)
			actionTree, err := nestActions(actions)
			if err != nil {
				log.Fatalln(err)
			}
			return actionTree
			// fmt.Println("\nactionTree")
			// fmt.Println(actionTree)
			// jsonDataPretty, err := json.MarshalIndent(actionTree, "", "  ")
			// if err != nil {
			// 	log.Fatalf("Error marshaling to pretty JSON: %v", err)
			// }

			// fmt.Println("\n--- Pretty-Printed JSON Output ---")
			// fmt.Println(string(jsonDataPretty))
		}
	} else {
		fmt.Println("Handle as none")
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
	Keyword string `json:"keyword"`
	Elapsed int    `json:"elapsed"` // total time elapsed when action ended
}

type Action struct {
	BaseAction
	Description string `json:"description"`
	Duration    int    `json:"duration"` // duration of action (optional)
	Children    any    `json:"children"` // nested children (optional)
}

func parseActions(baseActions []BaseAction) []Action {
	var actions []Action
	descMap := getDescriptionMap()
	for i, baseAction := range baseActions {
		duration := 0
		if i != 0 {
			duration = baseAction.Elapsed - baseActions[i-1].Elapsed
		}
		action := Action{
			BaseAction:  baseAction,
			Description: descMap[baseAction.Keyword],
			Duration:    duration,
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

func nestActions(actions []Action) (Action, error) {
	firstAction := actions[0]
	lastAction := actions[len(actions)-1]
	if firstAction.Keyword == "TS" && lastAction.Keyword == "TC" {
		children := actions[1 : len(actions)-1]
		for i, value := range children {
			if value.Keyword == "TS" || value.Keyword == "TC" {
				log.Fatalf("Log contains more than one transaction, %s found on index %d", value.Keyword, i)
			}
		}
		nestedChildren, err := nestActionsByProcessingRules(children)
		if err != nil {
			log.Fatalln(err)
		}
		transactionBase := BaseAction{
			Keyword: "Transaction",
			Elapsed: lastAction.Elapsed,
		}
		return Action{
			BaseAction:  transactionBase,
			Description: "TODO Custom Description",
			Duration:    lastAction.Elapsed - firstAction.Elapsed,
			Children:    nestedChildren,
		}, nil
	}
	return Action{}, errors.New("log does not start and end with TS and TC respectively")
}

func nestActionsByProcessingRules(actions []Action) ([]Action, error) {
	var err error
	var startTracker []int
	var childrenActions []Action
	startKeyword := "PS"
	endKeyword := "PC"
	for i, action := range actions {
		if action.Keyword == startKeyword {
			startTracker = append(startTracker, i)
			// fmt.Println("tracker")
			// fmt.Println(startTracker)
		}
		if action.Keyword == endKeyword {
			if len(startTracker) == 1 {
				firstStart := startTracker[0]
				startTracker = startTracker[:len(startTracker)-1]
				childrenSlice := actions[firstStart+1 : i]

				for _, childAction := range childrenSlice {
					if childAction.Keyword == "PS" || childAction.Keyword == "PC" {
						childrenSlice, err = nestActionsByProcessingRules(childrenSlice)
						if err != nil {
							log.Fatalln(err)
						}
					}
				}
				processingRuleBase := BaseAction{
					Keyword: "Processing Rule",
					Elapsed: action.Elapsed,
				}
				processingRuleAction := Action{
					BaseAction:  processingRuleBase,
					Description: "TODO Custom Description",
					Duration:    action.Elapsed - actions[firstStart].Elapsed,
					Children:    childrenSlice,
				}

				childrenActions = append(childrenActions, processingRuleAction)
			} else {
				startTracker = startTracker[:len(startTracker)-1]
			}
		}
		if action.Keyword != endKeyword && len(startTracker) == 0 {
			childrenActions = append(childrenActions, action)
		}
		// fmt.Printf("children %d", i)
		// fmt.Println(childrenActions)
	}

	return childrenActions, err
}
