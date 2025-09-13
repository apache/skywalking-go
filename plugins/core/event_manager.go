// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package core

import (
	"strconv"
	"sync"

	"github.com/pkg/errors"
)

type BaseEvent string

type ComplexEvent string

type LogicOp int

const (
	OpAnd     LogicOp = iota // AND: all conditions must be true
	OpOr                     // OR: at least one condition must be true
	OpNothing                // Do nothing, used for initial rules
)

type Rule struct {
	Event BaseEvent
	Op    LogicOp
	IsNot bool
}

// Expression node (used to build logical expression trees)
type ExprNode struct {
	Rules []Rule
	Event ComplexEvent
}

// EventManager manages event states and logical rules
type EventManager struct {
	mu              sync.RWMutex
	BaseEventStatus map[BaseEvent]bool         // current status of base events (true=enabled, false=disabled)
	ComplexEvents   map[ComplexEvent]*ExprNode // logical expressions for complex events
}

// Create a new EventManager
func NewEventManager() *EventManager {
	return &EventManager{
		BaseEventStatus: make(map[BaseEvent]bool),
		ComplexEvents:   make(map[ComplexEvent]*ExprNode),
	}
}

// Register a base event with initial status
func (m *EventManager) RegisterBaseEvent(event BaseEvent, initialStatus bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.BaseEventStatus[event] = initialStatus
}

// Register a complex event with logical expression rules
func (m *EventManager) RegisterComplexEvent(targetEvent ComplexEvent, expr *ExprNode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ComplexEvents[targetEvent] = expr
}

// Update the status of a base event
func (m *EventManager) UpdateBaseEventStatus(event BaseEvent, status bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.BaseEventStatus[event]; !ok {
		return errors.New("event not registered")
	}
	m.BaseEventStatus[event] = status
	return nil
}

// Get the status of a base event
func (m *EventManager) GetBaseEventStatus(event BaseEvent) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status, ok := m.BaseEventStatus[event]
	if !ok {
		return false, errors.New("event not registered")
	}
	return status, nil
}

// Execute a complex event by evaluating its logical expression
func (m *EventManager) ExecuteComplexEvent(event ComplexEvent) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	expr, ok := m.ComplexEvents[event]
	if !ok {
		return false, errors.New("event not registered")
	}
	return m.evalExpr(expr)
}

// Recursively evaluate the logical expression
func (m *EventManager) evalExpr(node *ExprNode) (bool, error) {
	if len(node.Rules) == 0 {
		return false, errors.New("complex event has no rules")
	}

	// 1. Evaluate the first rule directly (with optional NOT operation)
	firstRule := node.Rules[0]
	currentResult, err := m.getRuleValue(firstRule)
	if err != nil {
		return false, err
	}

	// 2. From the second rule onward, combine results using logical operators
	for i := 1; i < len(node.Rules); i++ {
		rule := node.Rules[i]
		// Get the value of the current rule (with optional NOT)
		ruleValue, err := m.getRuleValue(rule)
		if err != nil {
			return false, err
		}
		switch rule.Op {
		case OpAnd:
			currentResult = currentResult && ruleValue
		case OpOr:
			currentResult = currentResult || ruleValue
		default:
			return false, errors.New("invalid logic op: " + strconv.Itoa(int(rule.Op)))
		}
	}

	return currentResult, nil
}

// Get the value of a base event for a rule (with optional NOT)
func (m *EventManager) getRuleValue(rule Rule) (bool, error) {
	baseStatus, ok := m.BaseEventStatus[rule.Event]
	if !ok {
		return false, errors.New("base event not registered: " + string(rule.Event))
	}

	// Apply NOT operator if specified
	if rule.IsNot {
		return !baseStatus, nil
	}

	// Otherwise return the base event status directly
	return baseStatus, nil
}
