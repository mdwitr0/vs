package extractor

import "regexp"

var validators = map[IDType]*regexp.Regexp{
	IDKinopoisk: regexp.MustCompile(`^\d{1,8}$`),
	IDIMDb:      regexp.MustCompile(`^tt\d{7,10}$`),
	IDTMDB:      regexp.MustCompile(`^\d{1,7}$`),
	IDMAL:       regexp.MustCompile(`^\d{1,8}$`),
	IDShikimori: regexp.MustCompile(`^\d{1,8}$`),
}

func ValidateID(idType IDType, value string) bool {
	if value == "" {
		return false
	}
	v, ok := validators[idType]
	if !ok {
		return false
	}
	return v.MatchString(value)
}
