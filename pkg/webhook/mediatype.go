package webhook

import (
	"fmt"
	"strings"
)

const (
	mediaTypeFormat        = "application/external.dns.webhook+json;"
	supportedMediaVersions = "1"
)

var mediaTypeVersion1 = mediaTypeVersion("1")

type mediaType string

func mediaTypeVersion(v string) mediaType {
	return mediaType(mediaTypeFormat + "version=" + v)
}

func (m mediaType) Is(headerValue string) bool {
	return string(m) == headerValue
}

func checkAndGetMediaTypeHeaderValue(value string) (string, error) {
	for _, v := range strings.Split(supportedMediaVersions, ",") {
		if mediaTypeVersion(v).Is(value) {
			return v, nil
		}
	}

	versions := strings.Split(supportedMediaVersions, ",")
	supportedMediaTypes := make([]string, len(versions))
	for i, v := range versions {
		supportedMediaTypes[i] = string(mediaTypeVersion(v))
	}
	supportedMediaTypesString := strings.Join(supportedMediaTypes, ", ")

	return "", fmt.Errorf("unsupported media type version: '%s'. Supported media types are: '%s'", value, supportedMediaTypesString)
}
