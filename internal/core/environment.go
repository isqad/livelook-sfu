package core

type Environment string

const (
	DevelopmentEnv Environment = "development"
	ProductionEnv  Environment = "production"
)

func (e Environment) IsProduction() bool {
	return e == ProductionEnv
}

func (e Environment) IsDevelopment() bool {
	return e == DevelopmentEnv
}
