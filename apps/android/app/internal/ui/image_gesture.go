package ui

import (
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/io/pointer"
)

type imageGestureState struct {
	points        map[pointer.ID]f32.Point
	last          f32.Point
	startCenter   f32.Point
	startDistance float32
	startScale    float32
	startPan      image.Point
}

func (s *imageGestureState) apply(e pointer.Event, scale float32, pan image.Point) (float32, image.Point) {
	if s.points == nil {
		s.points = make(map[pointer.ID]f32.Point)
	}
	switch e.Kind {
	case pointer.Press:
		s.points[e.PointerID] = e.Position
		if len(s.points) == 1 {
			s.startScale, s.startPan, s.last = scale, pan, e.Position
		} else if len(s.points) == 2 {
			s.startScale, s.startPan = scale, pan
			ids := imageGesturePointerIDs(s.points)
			s.startCenter = imageGestureCenter(s.points[ids[0]], s.points[ids[1]])
			s.startDistance = imageGestureDistance(s.points[ids[0]], s.points[ids[1]])
		}
	case pointer.Drag, pointer.Move:
		previous, ok := s.points[e.PointerID]
		if !ok {
			return scale, pan
		}
		s.points[e.PointerID] = e.Position
		if len(s.points) >= 2 {
			ids := imageGesturePointerIDs(s.points)
			first, second := s.points[ids[0]], s.points[ids[1]]
			distance := imageGestureDistance(first, second)
			if s.startDistance > 0 {
				scale = clampImageScale(s.startScale * distance / s.startDistance)
			}
			center := imageGestureCenter(first, second)
			pan = image.Pt(
				s.startPan.X+int(math.Round(float64(center.X-s.startCenter.X))),
				s.startPan.Y+int(math.Round(float64(center.Y-s.startCenter.Y))),
			)
		} else if scale > 1 {
			pan = image.Pt(
				pan.X+int(math.Round(float64(e.Position.X-previous.X))),
				pan.Y+int(math.Round(float64(e.Position.Y-previous.Y))),
			)
		}
		s.last = e.Position
	case pointer.Release:
		delete(s.points, e.PointerID)
		s.resetBaseline(scale, pan)
	case pointer.Cancel:
		s.reset()
	}
	return scale, pan
}

func clampImageScale(scale float32) float32 {
	if scale < minImageScale {
		return minImageScale
	}
	if scale > maxImageScale {
		return maxImageScale
	}
	return scale
}

func clampImagePan(pan, size, viewport image.Point) image.Point {
	maxX := max(0, (size.X-viewport.X)/2)
	maxY := max(0, (size.Y-viewport.Y)/2)
	return image.Pt(clampInt(pan.X, -maxX, maxX), clampInt(pan.Y, -maxY, maxY))
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func imageGesturePointerIDs(points map[pointer.ID]f32.Point) []pointer.ID {
	ids := make([]pointer.ID, 0, len(points))
	for id := range points {
		ids = append(ids, id)
	}
	if len(ids) > 1 && ids[1] < ids[0] {
		ids[0], ids[1] = ids[1], ids[0]
	}
	return ids
}

func imageGestureCenter(a, b f32.Point) f32.Point {
	return f32.Pt((a.X+b.X)/2, (a.Y+b.Y)/2)
}

func imageGestureDistance(a, b f32.Point) float32 {
	return float32(math.Hypot(float64(b.X-a.X), float64(b.Y-a.Y)))
}

func (s *imageGestureState) resetBaseline(scale float32, pan image.Point) {
	s.startScale, s.startPan = scale, pan
	s.startDistance = 0
	if len(s.points) == 1 {
		ids := imageGesturePointerIDs(s.points)
		s.last = s.points[ids[0]]
		s.startCenter = s.last
	}
}

func (s *imageGestureState) reset() {
	s.points = nil
	s.last = f32.Point{}
	s.startCenter = f32.Point{}
	s.startDistance = 0
}
