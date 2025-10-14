package installable

import (
	"github.com/kyma-project/cfapi/api/v1alpha1"
	"k8s.io/client-go/tools/record"
)

type EventType string

const (
	EventWarning EventType = "Warning"
	EventNormal  EventType = "Normal"
)

//counterfeiter:generate -o fake -fake-name EventRecorder . EventRecorder
type EventRecorder interface {
	Event(eventtype EventType, reason, message string)
}

type CFAPIEventRecorder struct {
	recorder record.EventRecorder
	cfAPI    *v1alpha1.CFAPI
}

func NewCFAPIEventRecorder(recorder record.EventRecorder, cfAPI *v1alpha1.CFAPI) *CFAPIEventRecorder {
	return &CFAPIEventRecorder{recorder: recorder, cfAPI: cfAPI}
}

func (r *CFAPIEventRecorder) Event(eventtype EventType, reason, message string) {
	r.recorder.Event(r.cfAPI, string(eventtype), reason, message)
}
