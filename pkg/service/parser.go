package service

import "digger/pkg/domain"

type Parser struct {
}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(event *domain.Event, conf *domain.DiggerConfig) (*domain.ParsedEvent, error) {
	// TODO: to implement
	// Parser:
	// Add the right runner for each project
	// Replace commands by the right actions type ["plan", "apply"]
	return &domain.ParsedEvent{}, nil
}
