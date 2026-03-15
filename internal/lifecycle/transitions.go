package lifecycle

import (
	"fmt"

	"github.com/akhmanov/taskman/internal/model"
)

type TransitionInput struct {
	Actor      string
	ReasonType string
	Reason     string
	ResumeWhen string
	Summary    string
}

type transitionSpec struct {
	From []model.Status
	To   model.Status
}

var transitionSpecs = map[string]transitionSpec{
	"plan": {
		From: []model.Status{model.StatusBacklog},
		To:   model.StatusPlanned,
	},
	"start": {
		From: []model.Status{model.StatusPlanned},
		To:   model.StatusInProgress,
	},
	"pause": {
		From: []model.Status{model.StatusInProgress},
		To:   model.StatusPaused,
	},
	"resume": {
		From: []model.Status{model.StatusPaused},
		To:   model.StatusInProgress,
	},
	"complete": {
		From: []model.Status{model.StatusPlanned, model.StatusInProgress, model.StatusPaused},
		To:   model.StatusDone,
	},
	"cancel": {
		From: []model.Status{model.StatusBacklog, model.StatusPlanned, model.StatusInProgress, model.StatusPaused},
		To:   model.StatusCanceled,
	},
	"reopen": {
		From: []model.Status{model.StatusDone, model.StatusCanceled},
		To:   model.StatusPlanned,
	},
}

func getTransitionSpec(verb string) (transitionSpec, error) {
	spec, ok := transitionSpecs[verb]
	if !ok {
		return transitionSpec{}, fmt.Errorf("unknown transition %q", verb)
	}
	return spec, nil
}

func validateTransitionAllowed(current model.Status, verb string) (transitionSpec, error) {
	spec, err := getTransitionSpec(verb)
	if err != nil {
		return transitionSpec{}, err
	}
	for _, allowed := range spec.From {
		if current == allowed {
			return spec, nil
		}
	}
	return transitionSpec{}, fmt.Errorf("transition %q is not allowed from status %q", verb, current)
}

func ValidateTransitionForCLI(current model.Status, verb string) (model.Status, error) {
	spec, err := validateTransitionAllowed(current, verb)
	if err != nil {
		return "", err
	}
	return spec.To, nil
}

func buildStatusDetail(target model.Status, input TransitionInput) (model.StatusDetail, error) {
	switch target {
	case model.StatusPaused:
		if input.ReasonType == "" || input.Reason == "" || input.ResumeWhen == "" {
			return model.StatusDetail{}, fmt.Errorf("pause requires --reason-type, --reason, and --resume-when")
		}
		return model.StatusDetail{ReasonType: input.ReasonType, Reason: input.Reason, ResumeWhen: input.ResumeWhen}, nil
	case model.StatusDone:
		if input.Summary == "" {
			return model.StatusDetail{}, fmt.Errorf("complete requires --summary")
		}
		return model.StatusDetail{Summary: input.Summary}, nil
	case model.StatusCanceled:
		if input.ReasonType == "" || input.Reason == "" {
			return model.StatusDetail{}, fmt.Errorf("cancel requires --reason-type and --reason")
		}
		return model.StatusDetail{ReasonType: input.ReasonType, Reason: input.Reason}, nil
	default:
		return model.StatusDetail{}, nil
	}
}
