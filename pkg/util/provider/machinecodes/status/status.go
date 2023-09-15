/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * This file was copied and modified from the github.com/grpc/grpc-go project
 * https://github.com/grpc/grpc-go/blob/v1.29.x/status/status.go
 *
 * Modifications Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved.
 *
 */

// Package status implements errors returned by MachineAPIs. MachineAPIs
// service handlers should return an error created by this package,
// and machineAPIs clients should expect a corresponding error to be
// returned from the RPC call.
//
// This package upholds the invariants that a non-nil error may not
// contain an OK code, and an OK code must result in a nil error.
package status

import (
	"fmt"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
)

// Status implements error and Status,
type Status struct {
	// The status code, which should be an enum value of
	// ../codes.Code
	code int32
	// A developer-facing error message, which should be in English. Any
	// user-facing error message should be localized and sent in the
	// [google.rpc.Status.details][google.rpc.Status.details] field, or localized
	// by the client.
	message string
	// cause captures the underlying error
	cause error
}

// Code returns the status code contained in status.
func (s *Status) Code() codes.Code {
	if s == nil {
		return codes.OK
	}
	return codes.Code(s.code)
}

// Message returns the message contained in status.
func (s *Status) Message() string {
	return s.message
}

// Cause returns the underlying error if captured.
func (s *Status) Cause() error {
	return s.cause
}

// Error returns the error message for the status.
// WARNING: There is an unwritten contract for anyone using status.Status. One MUST never change
// the message text. It expects error code to be in the first square brackets and error message in the next. Therefore,
// any change made here should never change that. Any square brackets added after code and error are ignored when parsing.
func (s *Status) Error() string {
	return fmt.Sprintf("machine codes error: code = [%s] message = [%s]", s.Code(), s.Message())
}

// New returns a Status representing c and msg.
func New(c codes.Code, msg string) *Status {
	return &Status{code: int32(c), message: msg}
}

// Error returns an error representing c and msg.  If c is OK, returns nil.
func Error(c codes.Code, msg string) error {
	return New(c, msg)
}

// WrapError creates an instance of status.Status wrapping the underlying cause along with the code and custom error message.
func WrapError(c codes.Code, msg string, cause error) *Status {
	return &Status{
		code:    int32(c),
		message: msg,
		cause:   cause,
	}
}

// FromError returns a Status representing err if it was produced from this
// package or has a method `GRPCStatus() *Status`. Otherwise, ok is false and a
// Status is returned with codes.Unknown and the original error message.
func FromError(err error) (s *Status, ok bool) {
	if err == nil {
		return nil, true
	}

	if matches, errInFind := findCodeAndMessage(err.Error()); errInFind == nil {
		code := codes.StringToCode(matches[0])
		return &Status{
			code:    int32(code),
			message: matches[1],
		}, true
	}

	return &Status{
		code:    int32(codes.Unknown),
		message: err.Error(),
		cause:   err,
	}, false
}

func findCodeAndMessage(encodedMsg string) ([]string, error) {
	var decoded []string
	var temp []rune
	counter := 0

	for _, char := range encodedMsg {
		switch char {
		case '[':
			counter++
			temp = append(temp, char)
		case ']':
			if counter > 0 {
				counter--
				temp = append(temp, char)
				if counter == 0 {
					tempString := string(temp)
					decoded = append(decoded, tempString[1:len(tempString)-1])
					temp = nil
				}
			}
		default:
			if counter > 0 {
				temp = append(temp, char)
			}
		}
	}

	if len(decoded) != 2 {
		return nil, fmt.Errorf("unable to decode for machine code error")
	}
	return decoded, nil
}
