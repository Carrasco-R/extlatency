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
		match := datapowerLogRegex.FindStringSubmatch(logStr)
		if len(match) > 0 {
			frontSideRawActions := strings.Trim(match[1], ", ")
			// fmt.Println(frontSideRawActions)
			frontSideRawActionsSplit := strings.Split(frontSideRawActions, ",")
			frontSideBaseActions := parseActionsBase(frontSideRawActionsSplit)
			// frontSideActions := parseActions(frontSideBaseActions)
			// fmt.Println(frontSideActions)

			backSideRawActions := strings.Trim(match[2], ", ")
			// fmt.Println(backSideRawActions)
			backSideRawActionsSplit := strings.Split(backSideRawActions, ",")
			backSideBaseActions := parseActionsBase(backSideRawActionsSplit)
			// backSideActions := parseActions(backSideBaseActions)
			// fmt.Println(backSideActions)
			frontSideActions, backSideActions := parseActionsDatapower(frontSideBaseActions, backSideBaseActions)
			actionTree, err := nestTreeDatapower(frontSideActions, backSideActions)
			if err != nil {
				log.Fatalln(err)
			}
			fmt.Println(actionTree)

			jsonDataPretty, err := json.MarshalIndent(actionTree, "", "  ")
			if err != nil {
				log.Fatalf("Error marshaling to pretty JSON: %v", err)
			}

			fmt.Println("\n--- Pretty-Printed JSON Output ---")
			fmt.Println(string(jsonDataPretty))
		}
	} else if apiGatewayLogRegex.MatchString(logStr) {
		fmt.Println("Handle as APIC Log")
		match := apiGatewayLogRegex.FindStringSubmatch(logStr)
		if len(match) > 0 {
			actionsRaw := strings.Trim(match[1], ", ")
			actionsRawSplit := strings.Split(actionsRaw, ",")
			rawActions := parseActionsBase(actionsRawSplit)
			// fmt.Println(rawAction)
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
		// fmt.Println("Handle as none")
		log.Fatal("Log format does not match any Datapower/APIC Format")
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

func parseActionsDatapower(baseFrontActions []BaseAction, baseBackActions []BaseAction) ([]Action, []Action) {
	var actions []Action
	descMap := getDescriptionMap()
	var baseActions []BaseAction
	baseActions = append(baseActions, baseFrontActions...)
	baseActions = append(baseActions, baseBackActions...)
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
	frontSideActions := actions[:len(baseFrontActions)-1]
	backSideActions := actions[len(baseFrontActions):]
	fmt.Println((backSideActions))
	return frontSideActions, backSideActions
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

		// check if any other children contain TS or TC
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
func nestTreeDatapower(frontSideActions []Action, backSideActions []Action) (Action, error) {
	firstAction := frontSideActions[0]
	lastAction := backSideActions[len(backSideActions)-1]
	if firstAction.Keyword == "TS" && lastAction.Keyword == "TC" {
		frontSideChildren := frontSideActions[1 : len(frontSideActions)-1]
		backSideChildren := backSideActions[0 : len(backSideActions)-2]

		// check if any other children contain TS or TC
		for i, val := range frontSideChildren {
			if val.Keyword == "TS" || val.Keyword == "TC" {
				log.Fatalf("Log contains more than one transaction, %s found on index %d", val.Keyword, i)
			}
		}
		for i, val := range backSideChildren {
			if val.Keyword == "TS" || val.Keyword == "TC" {
				log.Fatalf("Log contains more than one transaction, %s found on index %d", val.Keyword, i)
			}
		}

		frontSideBase := BaseAction{
			Keyword: "Front Side Processing",
			Elapsed: frontSideActions[len(frontSideActions)-1].Elapsed,
		}
		nestedFrontChildren, err := nestActionsByProcessingRules(frontSideChildren)
		if err != nil {
			log.Fatalln(err)
		}
		frontSideAction := Action{
			BaseAction:  frontSideBase,
			Description: "Front Side Processing",
			Duration:    frontSideBase.Elapsed,
			Children:    nestedFrontChildren,
		}
		transactionBase := BaseAction{
			Keyword: "Transaction",
			Elapsed: lastAction.Elapsed,
		}

		backSideBase := BaseAction{
			Keyword: "Back Side Processing",
			Elapsed: backSideActions[len(backSideActions)-1].Elapsed,
		}
		nestedBackChildren, err := nestActionsByProcessingRules(backSideChildren)
		if err != nil {
			log.Fatalln(err)
		}
		backSideAction := Action{
			BaseAction:  backSideBase,
			Description: "Back Side Processing",
			Duration:    backSideBase.Elapsed - frontSideBase.Elapsed,
			Children:    nestedBackChildren,
		}

		var children []Action
		children = append(children, frontSideAction)
		children = append(children, backSideAction)
		fmt.Println(children)
		return Action{
			BaseAction:  transactionBase,
			Description: "TODO Custom Description",
			Duration:    lastAction.Elapsed - firstAction.Elapsed,
			Children:    children,
		}, nil

	}
	fmt.Println(firstAction, lastAction)
	return Action{}, nil
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
