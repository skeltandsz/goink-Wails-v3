// Package deletion 定义删除前阻塞项的通用契约与组合器。
package deletion

import (
	"context"
	"fmt"
)

// EntityKind 标识待删除或引用来源的实体类型。
type EntityKind string

const (
	EntityChapter   EntityKind = "chapter"
	EntityCharacter EntityKind = "character"
	EntityLocation  EntityKind = "location"
	EntityStoryArc  EntityKind = "story_arc"
)

// Target 描述一次删除检查的目标实体。
type Target struct {
	NovelID int64
	Kind    EntityKind
	ID      int64
}

// Blocker 是阻止删除的一条入站关系。
// Kind、ID 与 Label 用于调用方展示并定位引用来源。
type Blocker struct {
	Kind  string `json:"kind"`
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

// Checker 查询某一种目标实体的删除阻塞项。
type Checker func(ctx context.Context, novelID, targetID int64) ([]Blocker, error)

// Registration 将一个 Checker 注册到目标实体类型。
type Registration struct {
	TargetKind EntityKind
	Checker    Checker
}

// For 为指定目标实体类型创建一条检查注册。
func For(targetKind EntityKind, checker Checker) Registration {
	return Registration{TargetKind: targetKind, Checker: checker}
}

// Guard 查询目标实体的全部删除阻塞项。
// Guard 构造后只读，可由多个调用方并发使用。
type Guard struct {
	checkers map[EntityKind][]Checker
}

// NewGuard 组合按目标实体类型注册的检查器。
func NewGuard(registrations ...Registration) *Guard {
	checkers := make(map[EntityKind][]Checker)
	for _, registration := range registrations {
		if registration.Checker == nil {
			continue
		}
		checkers[registration.TargetKind] = append(checkers[registration.TargetKind], registration.Checker)
	}
	return &Guard{checkers: checkers}
}

func (g *Guard) Blockers(ctx context.Context, target Target) ([]Blocker, error) {
	var blockers []Blocker
	for _, checker := range g.checkers[target.Kind] {
		items, err := checker(ctx, target.NovelID, target.ID)
		if err != nil {
			return nil, fmt.Errorf("check %s deletion blockers: %w", target.Kind, err)
		}
		blockers = append(blockers, items...)
	}
	return blockers, nil
}
