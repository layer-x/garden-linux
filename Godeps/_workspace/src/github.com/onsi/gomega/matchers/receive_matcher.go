package matchers

import (
	"fmt"
	"github.com/onsi/gomega/format"
	"reflect"
)

type ReceiveMatcher struct {
	Arg           interface{}
	receivedValue interface{}
}

func (matcher *ReceiveMatcher) Match(actual interface{}) (success bool, err error) {
	if !isChan(actual) {
		return false, fmt.Errorf("ReceiveMatcher expects a channel.  Got:\n%s", format.Object(actual, 1))
	}

	channelType := reflect.TypeOf(actual)
	channelValue := reflect.ValueOf(actual)

	if channelType.ChanDir() == reflect.SendDir {
		return false, fmt.Errorf("ReceiveMatcher matcher cannot be passed a send-only channel.  Got:\n%s", format.Object(actual, 1))
	}

	var subMatcher omegaMatcher
	var hasSubMatcher bool

	if matcher.Arg != nil {
		subMatcher, hasSubMatcher = (matcher.Arg).(omegaMatcher)
		if !hasSubMatcher {
			argType := reflect.TypeOf(matcher.Arg)
			if argType.Kind() != reflect.Ptr {
				return false, fmt.Errorf("Cannot assign a value from the channel:\n%s\nTo:\n%s\nYou need to pass a pointer!", format.Object(actual, 1), format.Object(matcher.Arg, 1))
			}

			assignable := channelType.Elem().AssignableTo(argType.Elem())
			if !assignable {
				return false, fmt.Errorf("Cannot assign a value from the channel:\n%s\nTo:\n%s", format.Object(actual, 1), format.Object(matcher.Arg, 1))
			}
		}
	}

	winnerIndex, value, open := reflect.Select([]reflect.SelectCase{
		reflect.SelectCase{Dir: reflect.SelectRecv, Chan: channelValue},
		reflect.SelectCase{Dir: reflect.SelectDefault},
	})

	var closed bool
	var didReceive bool
	if winnerIndex == 0 {
		closed = !open
		didReceive = open
	}

	if closed {
		return false, fmt.Errorf("ReceiveMatcher was given a closed channel:\n%s", format.Object(actual, 1))
	}

	if hasSubMatcher {
		if didReceive {
			matcher.receivedValue = value.Interface()
			return subMatcher.Match(matcher.receivedValue)
		} else {
			return false, fmt.Errorf("When passed a matcher, ReceiveMatcher's channel *must* receive something.")
		}
	}

	if didReceive {
		if matcher.Arg != nil {
			outValue := reflect.ValueOf(matcher.Arg)
			reflect.Indirect(outValue).Set(value)
		}

		return true, nil
	} else {
		return false, nil
	}
}

func (matcher *ReceiveMatcher) FailureMessage(actual interface{}) (message string) {
	subMatcher, hasSubMatcher := (matcher.Arg).(omegaMatcher)

	if hasSubMatcher {
		return subMatcher.FailureMessage(matcher.receivedValue)
	} else {
		return format.Message(actual, "to receive something")
	}
}

func (matcher *ReceiveMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	subMatcher, hasSubMatcher := (matcher.Arg).(omegaMatcher)

	if hasSubMatcher {
		return subMatcher.NegatedFailureMessage(matcher.receivedValue)
	} else {
		return format.Message(actual, "not to receive anything")
	}
}
