package store

import (
	"sort"
	"sync/atomic"

	"github.com/konorlevich/fitto-club/internal/content"
)

var genSeq atomic.Uint64

func nextGen() uint64 { return genSeq.Add(1) }

// SeedSnapshot builds a snapshot from the shipped seed data. It is what a
// fresh database is filled with, and what Memory serves.
func SeedSnapshot() *Snapshot {
	s := &Snapshot{
		Gen:         nextGen(),
		Memberships: append([]content.Membership(nil), content.SeedMemberships...),
		SingleVisit: content.SingleVisitPrice,
		Tags:        append([]content.Tag(nil), content.SeedTags...),
		Coaches:     append([]content.Coach(nil), content.SeedCoaches...),
		Reviews:     append([]content.Review(nil), content.SeedReviews...),
		Classes:     append([]content.Class(nil), content.SeedClasses...),
		Massage:     append([]content.MassageService(nil), content.SeedMassage...),
		Hours:       content.SeedHours,
		SlugHistory: map[string]string{},
		LastMod:     map[string]string{},
	}
	sort.SliceStable(s.Coaches, func(i, j int) bool { return s.Coaches[i].Sort < s.Coaches[j].Sort })
	sort.SliceStable(s.Tags, func(i, j int) bool { return s.Tags[i].Sort < s.Tags[j].Sort })
	return s
}

// Memory is a read-only store over a fixed snapshot.
type Memory struct{ snap atomic.Pointer[Snapshot] }

func NewMemory(s *Snapshot) *Memory {
	m := &Memory{}
	m.snap.Store(s)
	return m
}

func (m *Memory) Current() *Snapshot { return m.snap.Load() }
func (m *Memory) Close() error       { return nil }
func (m *Memory) UploadDir() string  { return "" }
