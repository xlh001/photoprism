package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/dustin/go-humanize/english"
	"github.com/urfave/cli/v2"

	"github.com/photoprism/photoprism/internal/ai/vision"
	"github.com/photoprism/photoprism/internal/config"
	"github.com/photoprism/photoprism/pkg/clean"
	"github.com/photoprism/photoprism/pkg/txt/report"
)

// VisionListCommand configures the command name, flags, and action.
var VisionListCommand = &cli.Command{
	Name:   "ls",
	Usage:  "Lists the configured computer vision models",
	Flags:  report.CliFlags,
	Action: visionListAction,
}

// endpointRedacted is what a credential in a displayed endpoint is replaced with. It matches
// what net/url writes for a password, so a redacted query reads like a redacted userinfo.
const endpointRedacted = "xxxxx"

// credentialParams are the query parameter names whose value is treated as a credential.
// Matched as substrings of the lowercased name, so "X-Api-Key" and "access_token" are covered.
var credentialParams = []string{"key", "token", "secret", "password", "passwd", "pwd", "auth", "credential", "sig", "signature"}

// credentialParam reports whether a query parameter name is one whose value must not be shown.
func credentialParam(name string) bool {
	name = strings.ToLower(name)

	for _, s := range credentialParams {
		if strings.Contains(name, s) {
			return true
		}
	}

	return false
}

// endpointUriRedacted returns a service URI without the credentials it may carry, in the
// userinfo or in a query parameter, and an empty string when it cannot be parsed.
//
// url.Redacted covers the userinfo only, while an endpoint commonly authenticates through a
// query parameter instead, so both are removed before the value is displayed.
func endpointUriRedacted(s string) string {
	s = strings.TrimSpace(s)

	if s == "" {
		return ""
	}

	u, err := url.Parse(s)

	if err != nil {
		return ""
	}

	if q := u.Query(); len(q) > 0 {
		redacted := false

		for name, values := range q {
			if !credentialParam(name) {
				continue
			}

			for i := range values {
				values[i] = endpointRedacted
			}

			q[name] = values
			redacted = true
		}

		if redacted {
			u.RawQuery = q.Encode()
		}
	}

	return u.Redacted()
}

// visionEndpoint renders a service endpoint for display, without the credentials that
// Service.Endpoint injects into the URL for the request itself.
func visionEndpoint(uri, method string) string {
	if uri == "" || method == "" {
		return ""
	}

	if redacted := endpointUriRedacted(clean.Uri(uri)); redacted != "" {
		uri = redacted
	} else {
		// An unparsable URI is shown as a placeholder: it may still carry credentials.
		uri = "?"
	}

	return fmt.Sprintf("%s %s", method, uri)
}

// visionListAction displays the configured computer vision models.
func visionListAction(ctx *cli.Context) error {
	return CallWithDependencies(ctx, func(conf *config.Config) error {
		var rows [][]string

		cols := []string{
			"Model",
			"Type",
			"Engine",
			"Endpoint",
			"Format",
			"Normalize",
			"Resolution",
			"Options",
			"Schedule",
			"Status",
		}

		// Show log message.
		log.Infof("found %s", english.Plural(len(vision.Config.Models), "model", "models"))

		if n := len(vision.Config.Models); n == 0 {
			return nil
		} else {
			rows = make([][]string, n)
		}

		// Display report.
		for i, model := range vision.Config.Models {
			modelUri, modelMethod := model.Endpoint()
			tags := ""

			name, _, _ := model.GetModel()

			if model.TensorFlow != nil && model.TensorFlow.Tags != nil {
				tags = strings.Join(model.TensorFlow.Tags, ", ")
			}

			var options []byte
			if o := model.GetOptions(); o != nil {
				options, _ = json.Marshal(*o)
			}

			var format string

			if modelUri != "" && modelMethod != "" {
				if f := model.EndpointRequestFormat(); f != "" {
					format = f
				}
			}

			if responseFormat := model.GetFormat(); responseFormat != "" {
				if format != "" {
					format = fmt.Sprintf("%s:%s", format, responseFormat)
				} else {
					format = responseFormat
				}
			}

			if format == "" && model.Default {
				format = "default"
			}

			var run string

			if run = model.RunType(); run == "" {
				run = "auto"
			}

			engine := model.EngineName()

			// Normalization only runs on the response of a remote labels model, and the
			// effective mode is shown because an unset value resolves to one.
			var normalize string

			if model.Type == vision.ModelTypeLabels && modelUri != "" && modelMethod != "" {
				normalize = model.GetNormalize()
			}

			rows[i] = []string{
				name,
				model.Type,
				engine,
				visionEndpoint(modelUri, modelMethod),
				format,
				normalize,
				fmt.Sprintf("%d", model.Resolution),
				report.Bool(model.TensorFlow != nil, fmt.Sprintf(`{"tags":"%s"}`, tags), string(options)),
				run,
				report.Bool(model.Disabled, report.Disabled, report.Enabled),
			}
		}

		result, err := report.RenderFormat(rows, cols, report.CliFormat(ctx))

		fmt.Printf("\n%s\n", result)

		return err
	})
}
