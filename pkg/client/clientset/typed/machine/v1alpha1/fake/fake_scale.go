package fake

// FakeScales implements ScaleInterface
type FakeScales struct {
	Fake *FakeMachineV1alpha1
	ns   string
}
