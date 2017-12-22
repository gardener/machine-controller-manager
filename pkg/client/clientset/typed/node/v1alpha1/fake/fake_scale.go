package fake

// FakeScales implements ScaleInterface
type FakeScales struct {
	Fake *FakeNodeV1alpha1
	ns   string
}
