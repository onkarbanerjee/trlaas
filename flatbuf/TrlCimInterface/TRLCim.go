// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package TrlCimInterface

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type TRLCim struct {
	_tab flatbuffers.Table
}

func GetRootAsTRLCim(buf []byte, offset flatbuffers.UOffsetT) *TRLCim {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &TRLCim{}
	x.Init(buf, n+offset)
	return x
}

func (rcv *TRLCim) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *TRLCim) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *TRLCim) MTCILHeader(obj *TrlData, j int) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		x := rcv._tab.Vector(o)
		x += flatbuffers.UOffsetT(j) * 4
		x = rcv._tab.Indirect(x)
		obj.Init(rcv._tab.Bytes, x)
		return true
	}
	return false
}

func (rcv *TRLCim) MTCILHeaderLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *TRLCim) Trl(j int) byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.GetByte(a + flatbuffers.UOffsetT(j*1))
	}
	return 0
}

func (rcv *TRLCim) TrlLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *TRLCim) TrlBytes() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *TRLCim) MutateTrl(j int, n byte) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.MutateByte(a+flatbuffers.UOffsetT(j*1), n)
	}
	return false
}

func TRLCimStart(builder *flatbuffers.Builder) {
	builder.StartObject(2)
}
func TRLCimAddMTCILHeader(builder *flatbuffers.Builder, MTCILHeader flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, flatbuffers.UOffsetT(MTCILHeader), 0)
}
func TRLCimStartMTCILHeaderVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func TRLCimAddTrl(builder *flatbuffers.Builder, Trl flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, flatbuffers.UOffsetT(Trl), 0)
}
func TRLCimStartTrlVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(1, numElems, 1)
}
func TRLCimEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}