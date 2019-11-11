
import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// LabelTriggeredExtendedJob allows customization of labels triggers
func (c *Catalog) LabelTriggeredExtendedJob(name string, state ejv1.PodState, ml labels.Set, me []*ejv1.Requirement, cmd []string) *ejv1.ExtendedJob {
	return &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: "podstate",
				PodState: &ejv1.PodStateTrigger{
					When: state,
					Selector: &ejv1.Selector{
						MatchLabels:      &ml,
						MatchExpressions: me,
					},
				},
			},
			Template: c.CmdPodTemplate(cmd),
		},
	}
}

// LongRunningExtendedJob has a longer sleep time
func (c *Catalog) LongRunningExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateReady,
		map[string]string{"key": "value"},
		[]*ejv1.Requirement{},
		[]string{"sleep", "15"},
	)
}

// OnDeleteExtendedJob runs for deleted pods
func (c *Catalog) OnDeleteExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateDeleted,
		map[string]string{"key": "value"},
		[]*ejv1.Requirement{},
		[]string{"sleep", "1"},
	)
}

// MatchExpressionExtendedJob uses Matchexpressions for matching
func (c *Catalog) MatchExpressionExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateReady,
		map[string]string{},
		[]*ejv1.Requirement{
			{Key: "env", Operator: "in", Values: []string{"production"}},
		},
		[]string{"sleep", "1"},
	)
}

// ComplexMatchExtendedJob uses MatchLabels and MatchExpressions
func (c *Catalog) ComplexMatchExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateReady,
		map[string]string{"key": "value"},
		[]*ejv1.Requirement{
			{Key: "env", Operator: "in", Values: []string{"production"}},
		},
		[]string{"sleep", "1"},
	)
}
