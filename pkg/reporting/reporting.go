package reporting

import "digger/pkg/ci"

type Reporter interface {
	Report(report string) error
}

type CiReporter struct {
	CiService ci.CIService
	PrNumber  int
}

func (ciReporter *CiReporter) Report(report string) error {
	return ciReporter.CiService.PublishComment(ciReporter.PrNumber, report)
}
