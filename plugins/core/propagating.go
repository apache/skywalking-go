// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package core

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	Header                        string = "sw8"
	HeaderCorrelation             string = "sw8-correlation"
	headerLen                     int    = 8
	splitToken                    string = "-"
	correlationSplitToken         string = ","
	correlationKeyValueSplitToken string = ":"
)

var (
	errEmptyHeader                = errors.New("empty header")
	errInsufficientHeaderEntities = errors.New("insufficient header entities")
)

type SpanContext struct {
	TraceID               string            `json:"trace_id"`
	ParentSegmentID       string            `json:"parent_segment_id"`
	ParentService         string            `json:"parent_service"`
	ParentServiceInstance string            `json:"parent_service_instance"`
	ParentEndpoint        string            `json:"parent_endpoint"`
	AddressUsedAtClient   string            `json:"address_used_at_client"`
	ParentSpanID          int32             `json:"parent_span_id"`
	Sample                int8              `json:"sample"`
	Valid                 bool              `json:"valid"`
	CorrelationContext    map[string]string `json:"correlation_context"`
}

func (s *SpanContext) GetTraceID() string {
	return s.TraceID
}

func (s *SpanContext) GetParentSegmentID() string {
	return s.ParentSegmentID
}

func (s *SpanContext) GetParentService() string {
	return s.ParentService
}

func (s *SpanContext) GetParentServiceInstance() string {
	return s.ParentServiceInstance
}

func (s *SpanContext) GetParentEndpoint() string {
	return s.ParentEndpoint
}

func (s *SpanContext) GetAddressUsedAtClient() string {
	return s.AddressUsedAtClient
}

func (s *SpanContext) GetParentSpanID() int32 {
	return s.ParentSpanID
}

// Decode all SpanContext data from Extractor
func (s *SpanContext) Decode(extractor func(headerKey string) (string, error)) error {
	s.Valid = false
	// sw8
	err := s.decode(extractor, Header, s.DecodeSW8)
	if err != nil {
		return err
	}

	// correlation
	err = s.decode(extractor, HeaderCorrelation, s.DecodeSW8Correlation)
	if err != nil {
		return err
	}
	return nil
}

// Encode all SpanContext data to Injector
func (s *SpanContext) Encode(injector func(headerKey, headerValue string) error) error {
	// sw8
	err := injector(Header, s.EncodeSW8())
	if err != nil {
		return err
	}
	// correlation
	err = injector(HeaderCorrelation, s.EncodeSW8Correlation())
	if err != nil {
		return err
	}
	return nil
}

// DecodeSW6 converts string header to SpanContext
func (s *SpanContext) DecodeSW8(header string) error {
	if header == "" {
		return errEmptyHeader
	}
	hh := strings.Split(header, splitToken)
	if len(hh) < headerLen {
		return errors.WithMessagef(errInsufficientHeaderEntities, "header string: %s", header)
	}
	sample, err := strconv.ParseInt(hh[0], 10, 8)
	if err != nil {
		return errors.Errorf("str to int8 error %s", hh[0])
	}
	s.Sample = int8(sample)
	s.TraceID, err = decodeBase64(hh[1])
	if err != nil {
		return errors.Wrap(err, "trace id parse error")
	}
	s.ParentSegmentID, err = decodeBase64(hh[2])
	if err != nil {
		return errors.Wrap(err, "parent segment id parse error")
	}
	s.ParentSpanID, err = stringConvertInt32(hh[3])
	if err != nil {
		return errors.Wrap(err, "parent span id parse error")
	}
	s.ParentService, err = decodeBase64(hh[4])
	if err != nil {
		return errors.Wrap(err, "parent service parse error")
	}
	s.ParentServiceInstance, err = decodeBase64(hh[5])
	if err != nil {
		return errors.Wrap(err, "parent service instance parse error")
	}
	s.ParentEndpoint, err = decodeBase64(hh[6])
	if err != nil {
		return errors.Wrap(err, "parent endpoint parse error")
	}
	s.AddressUsedAtClient, err = decodeBase64(hh[7])
	if err != nil {
		return errors.Wrap(err, "network address parse error")
	}
	s.Valid = true
	return nil
}

// EncodeSW6 converts SpanContext to string header
func (s *SpanContext) EncodeSW8() string {
	return strings.Join([]string{
		fmt.Sprint(s.Sample),
		encodeBase64(s.TraceID),
		encodeBase64(s.ParentSegmentID),
		fmt.Sprint(s.ParentSpanID),
		encodeBase64(s.ParentService),
		encodeBase64(s.ParentServiceInstance),
		encodeBase64(s.ParentEndpoint),
		encodeBase64(s.AddressUsedAtClient),
	}, "-")
}

// DecodeSW8Correlation converts correlation string header to SpanContext
func (s *SpanContext) DecodeSW8Correlation(header string) error {
	s.CorrelationContext = make(map[string]string)
	if header == "" {
		return nil
	}

	hh := strings.Split(header, correlationSplitToken)
	for inx := range hh {
		keyValues := strings.Split(hh[inx], correlationKeyValueSplitToken)
		if len(keyValues) != 2 {
			continue
		}
		decodedKey, err := decodeBase64(keyValues[0])
		if err != nil {
			continue
		}
		decodedValue, err := decodeBase64(keyValues[1])
		if err != nil {
			continue
		}

		s.CorrelationContext[decodedKey] = decodedValue
	}
	return nil
}

// EncodeSW8Correlation converts correlation to string header
func (s *SpanContext) EncodeSW8Correlation() string {
	if len(s.CorrelationContext) == 0 {
		return ""
	}

	content := make([]string, 0, len(s.CorrelationContext))
	for k, v := range s.CorrelationContext {
		content = append(content, fmt.Sprintf("%s%s%s", encodeBase64(k), correlationKeyValueSplitToken, encodeBase64(v)))
	}
	return strings.Join(content, correlationSplitToken)
}

func stringConvertInt32(str string) (int32, error) {
	i, err := strconv.ParseInt(str, 0, 32)
	return int32(i), err
}

func decodeBase64(str string) (string, error) {
	ret, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func encodeBase64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func (s *SpanContext) decode(extractor func(headerKey string) (string, error), headerKey string, decoder func(header string) error) error {
	val, err := extractor(headerKey)
	if err != nil {
		return err
	}
	if val == "" {
		return nil
	}
	err = decoder(val)
	if err != nil {
		return err
	}
	return nil
}
