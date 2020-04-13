package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/golang/gddo/httputil/header"
)

func unmarshal(w http.ResponseWriter, r *http.Request, data interface{}) (error, int) {
	value, _ := header.ParseValueAndParams(r.Header, "content-type")
	if value != "application/json" {
		return fmt.Errorf("content-type is not application/json"), http.StatusUnsupportedMediaType
	}

	defer r.Body.Close()
	// TODO - Max Size may need to be adjusted. Starting with 64K
	// Publish API only needs about 1K
	// leaving room for safetyNet attestation payloads.
	r.Body = http.MaxBytesReader(w, r.Body, 64000)

	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()

	if err := d.Decode(&data); err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalError *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntaxErr):
			return fmt.Errorf("malformed json at position %v", syntaxErr.Offset), http.StatusBadRequest
		case errors.Is(err, io.ErrUnexpectedEOF):
			return fmt.Errorf("malformed json"), http.StatusBadRequest
		case errors.As(err, &unmarshalError):
			return fmt.Errorf("invalid value %v at position %v", unmarshalError.Field, unmarshalError.Offset), http.StatusBadRequest
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("unknown field %s", fieldName), http.StatusBadRequest
		case errors.Is(err, io.EOF):
			return fmt.Errorf("body must not be empty"), http.StatusBadRequest
		case err.Error() == "http: request body too large":
			return err, http.StatusRequestEntityTooLarge
		default:
			return fmt.Errorf("failed to decode json %v", err), http.StatusInternalServerError
		}
	}
	if d.More() {
		return fmt.Errorf("body must contain only one JSON object"), http.StatusBadRequest
	}

	return nil, http.StatusOK
}
