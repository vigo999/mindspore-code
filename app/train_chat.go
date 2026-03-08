package main

import (
	"fmt"
	"strings"

	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/ui/model"
)

const (
	trainChatRetryMarker = "[[MSCLI_TRAIN_RETRY]]"
	trainChatStopMarker  = "[[MSCLI_TRAIN_STOP]]"
)

func (a *Application) processTrainChatInput(input string) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return
	}
	go a.runTrainChatTask(trimmed)
}

func (a *Application) runTrainChatTask(userPrompt string) {
	a.EventCh <- model.Event{Type: model.AgentThinking}

	task := loop.Task{
		ID:          generateTaskID(),
		Description: a.buildTrainChatTaskPrompt(userPrompt),
	}

	events, err := a.Engine.Run(task)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline") {
			errMsg = fmt.Sprintf("%s\n\nTip: The request timed out. Try /compact or /clear before asking for deeper /train analysis.", errMsg)
		}
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "Engine",
			Message:  errMsg,
		}
		return
	}

	retryRequested := false
	stopRequested := false

	for _, ev := range events {
		if ev.Type == loop.EventAgentReply {
			cleaned, retry, stop := extractTrainChatActions(ev.Message)
			retryRequested = retryRequested || retry
			stopRequested = stopRequested || stop
			if strings.TrimSpace(cleaned) == "" {
				switch {
				case stopRequested:
					cleaned = "Stopping the current /train workflow."
				case retryRequested:
					cleaned = "Applying the requested changes and retrying /train with the previous workflow."
				}
			}
			ev.Message = cleaned
		}

		uiEvent := a.convertEvent(ev)
		if uiEvent != nil {
			a.EventCh <- *uiEvent
		}
	}

	switch {
	case stopRequested:
		a.handleCommand("/train stop")
	case retryRequested:
		a.handleCommand("/train retry")
	}
}

func (a *Application) buildTrainChatTaskPrompt(userPrompt string) string {
	var sb strings.Builder
	sb.WriteString("You are inside the ms-cli /train analysis chat.\n\n")
	sb.WriteString("Current training dashboard snapshot:\n")
	sb.WriteString(a.trainChatContextSummary())
	sb.WriteString("\n\nUser request:\n")
	sb.WriteString(userPrompt)
	sb.WriteString("\n\nInstructions:\n")
	sb.WriteString("- Use the normal coding workflow to inspect results, edit files, and prepare follow-up changes.\n")
	sb.WriteString("- If the user explicitly wants the training workflow to be retried after your edits are complete, put this exact marker on its own final line: ")
	sb.WriteString(trainChatRetryMarker)
	sb.WriteString("\n")
	sb.WriteString("- If the user explicitly wants the current training workflow to stop, put this exact marker on its own final line: ")
	sb.WriteString(trainChatStopMarker)
	sb.WriteString("\n")
	sb.WriteString("- Do not emit /train slash commands yourself; use only the markers above when needed.\n")
	return sb.String()
}

func (a *Application) trainChatContextSummary() string {
	a.trainMu.Lock()
	defer a.trainMu.Unlock()

	var sb strings.Builder
	train := a.trainState

	sb.WriteString(fmt.Sprintf("run_id: %s\n", fallbackTrain(train.RunID, a.trainLastID, "-")))
	sb.WriteString(fmt.Sprintf("status: %s\n", fallbackTrain(train.Status, "-", "-")))
	if train.CurrentStage != "" {
		sb.WriteString(fmt.Sprintf("current_stage: %s\n", train.CurrentStage))
	}
	if len(a.trainLastArgs) > 0 {
		sb.WriteString(fmt.Sprintf("last_train_args: %s\n", strings.Join(a.trainLastArgs, " ")))
	}
	if train.FailedStage != "" || train.FailedHost != "" || train.Error != "" {
		sb.WriteString("failure:\n")
		if train.FailedStage != "" {
			sb.WriteString(fmt.Sprintf("  stage: %s\n", train.FailedStage))
		}
		if train.FailedHost != "" {
			sb.WriteString(fmt.Sprintf("  host: %s\n", train.FailedHost))
		}
		if train.Error != "" {
			sb.WriteString(fmt.Sprintf("  error: %s\n", strings.ReplaceAll(strings.TrimSpace(train.Error), "\n", " | ")))
		}
	}

	if len(train.HostOrder) == 0 {
		sb.WriteString("hosts: (none)\n")
	} else {
		sb.WriteString("hosts:\n")
		for _, hostName := range train.HostOrder {
			host := train.Hosts[hostName]
			if host == nil {
				continue
			}
			sb.WriteString(fmt.Sprintf("  - %s status=%s step=%s loss=%s throughput=%s grad_norm=%s model=%s\n",
				host.Name,
				host.Status,
				formatTrainMetricStep(host.Step, host.TotalStep),
				formatTrainMetricFloat(host.Loss, host.Step > 0 || host.Loss != 0, 5),
				formatTrainMetricFloat(host.Throughput, host.Throughput != 0, 2),
				formatTrainMetricFloat(host.GradNorm, host.GradNorm != 0, 4),
				fallbackTrain(host.Model, "-", "-"),
			))
		}
	}

	logs := train.RecentLogs
	if len(logs) > 8 {
		logs = logs[len(logs)-8:]
	}
	sb.WriteString("recent_logs:\n")
	if len(logs) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, line := range logs {
			sb.WriteString(fmt.Sprintf("  - %s\n", strings.TrimSpace(line)))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

func extractTrainChatActions(message string) (string, bool, bool) {
	lines := strings.Split(message, "\n")
	kept := make([]string, 0, len(lines))
	retry := false
	stop := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case trainChatRetryMarker:
			retry = true
			continue
		case trainChatStopMarker:
			stop = true
			continue
		default:
			kept = append(kept, line)
		}
	}

	return strings.TrimSpace(strings.Join(kept, "\n")), retry, stop
}

func fallbackTrain(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && value != "-" {
			return value
		}
	}
	if len(values) > 0 {
		return values[len(values)-1]
	}
	return "-"
}

func formatTrainMetricStep(step, total int) string {
	switch {
	case step > 0 && total > 0:
		return fmt.Sprintf("%d/%d", step, total)
	case total > 0:
		return fmt.Sprintf("0/%d", total)
	case step > 0:
		return fmt.Sprintf("%d", step)
	default:
		return "-"
	}
}

func formatTrainMetricFloat(v float64, ok bool, prec int) string {
	if !ok {
		return "-"
	}
	return fmt.Sprintf("%.*f", prec, v)
}
