package extlatency

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var datapowerLogRegex = regexp.MustCompile(`(?:ExtLatency: )(.*)(?: == )(.*)(\[.*\])$`) // must try first
var apiGatewayLogRegex = regexp.MustCompile(`(?:ExtLatency: )(.*)(\[.*\])$`)

func Parse(logStr string) (Action, error) {
	var actionTree Action
	descriptionMapFilePath := "descriptions.json"
	descMap, err := getDescriptionMap(descriptionMapFilePath)
	if err != nil {
		return Action{}, err
	}
	datapowerMatch := datapowerLogRegex.FindStringSubmatch(logStr)
	if len(datapowerMatch) > 0 {
		// Parse as DP Log
		frontSideRawActions := strings.Trim(datapowerMatch[1], ", ")
		frontSideRawActionsSplit := strings.Split(frontSideRawActions, ",")
		frontSideBaseActions, err := parseActionsBase(frontSideRawActionsSplit)
		if err != nil {
			return Action{}, err
		}

		backSideRawActions := strings.Trim(datapowerMatch[2], ", ")
		backSideRawActionsSplit := strings.Split(backSideRawActions, ",")
		backSideBaseActions, err := parseActionsBase(backSideRawActionsSplit)
		if err != nil {
			return Action{}, err
		}

		frontSideActions, backSideActions := parseActionsDatapower(frontSideBaseActions, backSideBaseActions, descMap)
		nestedDatapowerLogTree, err := nestTreeDatapower(frontSideActions, backSideActions)
		if err != nil {
			return Action{}, err
		}
		actionTree = nestedDatapowerLogTree
		
	} else {
		// Handle as APIC Log
		apiGatewayMatch := apiGatewayLogRegex.FindStringSubmatch(logStr)
		if len(apiGatewayMatch) > 0 {
			actionsRaw := strings.Trim(apiGatewayMatch[1], ", ")
			actionsRawSplit := strings.Split(actionsRaw, ",")
			rawActions, err := parseActionsBase(actionsRawSplit)
			if err != nil {
				return Action{}, err
			}
			actions := parseActions(rawActions, descMap)
			nestedTree, err := nestActions(actions)
			if err != nil {
				return Action{}, err
			}
			actionTree = nestedTree
		} else {
			return Action{}, fmt.Errorf("log does not match the format of extLatency logs")
		}
	}
	return actionTree, nil
}

func getDescriptionMap(filePath string) (map[string]string, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var data map[string]string
	err = json.Unmarshal(fileContent, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
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

func parseActions(baseActions []BaseAction, descMap map[string]string) []Action {
	var actions []Action
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

func parseActionsDatapower(baseFrontActions []BaseAction, baseBackActions []BaseAction, descMap map[string]string) ([]Action, []Action) {
	var actions []Action
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
	frontSideActions := actions[:len(baseFrontActions)]
	backSideActions := actions[len(baseFrontActions):]
	return frontSideActions, backSideActions
}

func parseActionsBase(actionsRawSplit []string) ([]BaseAction, error) {
	var actions []BaseAction
	for _, actionStrRaw := range actionsRawSplit {
		splitStrs := strings.Split(actionStrRaw, "=")
		keyword := splitStrs[0]
		elapsed, err := strconv.ParseInt(splitStrs[1], 0, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse action %s elapsed time as int", keyword)
		}
		action := BaseAction{keyword, int(elapsed)}
		actions = append(actions, action)
	}
	return actions, nil
}

func nestActions(actions []Action) (Action, error) {
	firstAction := actions[0]
	lastAction := actions[len(actions)-1]
	if firstAction.Keyword == "TS" && lastAction.Keyword == "TC" {
		children := actions[1 : len(actions)-1]

		// check if any other children contain TS or TC
		for i, value := range children {
			if value.Keyword == "TS" || value.Keyword == "TC" {
				return Action{}, fmt.Errorf("log contains more than one transaction, %s found on index %d", value.Keyword, i)
			}
		}
		nestedChildren, err := nestActionsByProcessingRules(children)
		if err != nil {
			return Action{}, err
		}
		transactionBase := BaseAction{
			Keyword: "Transaction",
			Elapsed: lastAction.Elapsed,
		}
		return Action{
			BaseAction:  transactionBase,
			Description: "Transaction",
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
		frontSideChildren := frontSideActions[1:]
		backSideChildren := backSideActions[:len(backSideActions)-1]

		// check if any other children contain TS or TC
		for i, val := range frontSideChildren {
			if val.Keyword == "TS" || val.Keyword == "TC" {
				return Action{}, fmt.Errorf("log contains more than one transaction, %s found on index %d", val.Keyword, i)
			}
		}
		for i, val := range backSideChildren {
			if val.Keyword == "TS" || val.Keyword == "TC" {
				return Action{}, fmt.Errorf("log contains more than one transaction, %s found on index %d", val.Keyword, i)
			}
		}

		frontSideBase := BaseAction{
			Keyword: "Front Side Processing",
			Elapsed: frontSideActions[len(frontSideActions)-1].Elapsed,
		}
		nestedFrontChildren, err := nestActionsByProcessingRules(frontSideChildren)
		if err != nil {
			return Action{}, err
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
			return Action{}, err
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
		return Action{
			BaseAction:  transactionBase,
			Description: "TODO Custom Description",
			Duration:    lastAction.Elapsed - firstAction.Elapsed,
			Children:    children,
		}, nil

	}
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
							return nil, err
						}
					}
				}
				processingRuleBase := BaseAction{
					Keyword: "Processing Rule",
					Elapsed: action.Elapsed,
				}
				processingRuleAction := Action{
					BaseAction:  processingRuleBase,
					Description: "Specifies the processing actions to apply to incoming documents",
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
	}

	return childrenActions, err
}
