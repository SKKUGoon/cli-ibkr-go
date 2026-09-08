package cli

import (
	"encoding/json"
	"fmt"
	"ibkr-go/ibkr"
	"os"
)

func (app *application) readAnswers(file, inline string) (ibkr.Answers, error) {
	answers := ibkr.DefaultAnswers()
	merge := func(raw []byte) error {
		var overrides map[string]json.RawMessage
		if err := json.Unmarshal(raw, &overrides); err != nil {
			return err
		}
		if overrides == nil {
			return fmt.Errorf("answers must be a JSON object")
		}
		for key, value := range overrides {
			var answer bool
			if string(value) == "null" || json.Unmarshal(value, &answer) != nil {
				return fmt.Errorf("answer %s must be boolean", key)
			}
			answers[key] = answer
		}
		return nil
	}
	base := app.environment["IBKR_ORDERS_ANSWER_JSON"]
	if base != "" {
		raw, err := os.ReadFile(base)
		if err == nil {
			if err = merge(raw); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		if err = merge(raw); err != nil {
			return nil, err
		}
	}
	if inline != "" {
		if err := merge([]byte(inline)); err != nil {
			return nil, err
		}
	}
	return answers, nil
}
