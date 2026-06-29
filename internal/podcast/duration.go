package podcast

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseDuration(value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("duration is required")
	}
	parts := strings.Split(value, ":")
	switch len(parts) {
	case 1:
		seconds, err := parseDurationPart(parts[0])
		if err != nil {
			return 0, err
		}
		return seconds, nil
	case 2:
		minutes, err := parseDurationPart(parts[0])
		if err != nil {
			return 0, err
		}
		seconds, err := parseDurationPart(parts[1])
		if err != nil {
			return 0, err
		}
		if seconds > 59 {
			return 0, fmt.Errorf("duration seconds out of range")
		}
		return minutes*60 + seconds, nil
	case 3:
		hours, err := parseDurationPart(parts[0])
		if err != nil {
			return 0, err
		}
		minutes, err := parseDurationPart(parts[1])
		if err != nil {
			return 0, err
		}
		seconds, err := parseDurationPart(parts[2])
		if err != nil {
			return 0, err
		}
		if minutes > 59 || seconds > 59 {
			return 0, fmt.Errorf("duration clock fields out of range")
		}
		return hours*3600 + minutes*60 + seconds, nil
	default:
		return 0, fmt.Errorf("invalid duration format")
	}
}

func parseDurationPart(value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("empty duration field")
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid duration field")
	}
	return parsed, nil
}
