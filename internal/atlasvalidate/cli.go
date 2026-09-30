package atlasvalidate

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
)

func RunCLI(args []string, stdout, stderr io.Writer) int {
	scoring := len(args) > 0 && args[0] == "score-tower"
	if scoring {
		args = args[1:]
	}
	flags := flag.NewFlagSet("atlas-validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	data := flags.String("data", "", "raw game-data directory")
	gameData := flags.String("game-data", "", "game-data directory")
	tower := flags.String("tower", "", "selected Tower JSON path")
	profile := flags.String("profile", "", "self-contained Profile directory")
	format := flags.String("format", "json", "output format: json or text")
	relations := flags.Bool("relations", false, "include resolved references and file backlinks in JSON output")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *gameData != "" {
		if *data != "" {
			fmt.Fprintln(stderr, "use only one of --data and --game-data")
			return 2
		}
		*data = *gameData
	}
	if (scoring && (*gameData == "" || *tower == "")) || (!scoring && *tower != "") || *data == "" || *profile == "" || flags.NArg() != 0 || (*format != "json" && *format != "text") || (*relations && *format != "json") {
		if scoring {
			fmt.Fprintln(stderr, "Usage: atlas-validator score-tower --game-data <path> --profile <path> --tower <path> [--format json|text] [--relations]")
			return 2
		}
		fmt.Fprintln(stderr, "Usage: atlas-validate --data <game-data> --profile <profile> [--format json|text] [--relations]")
		return 2
	}
	var result Report
	var status int
	if scoring {
		result, status = ScoreTower(*data, *profile, *tower, *relations)
	} else {
		result, status = Validate(*data, *profile, *relations)
	}
	if *format == "text" {
		if result.Score != nil {
			fmt.Fprintf(stdout, "score=%.2f/100; complete=%t\n", result.Score.Points, result.Score.Complete)
		}
		for _, error := range result.Errors {
			fmt.Fprintf(stdout, "%s#%s: %s: %s\n", error.File, error.Pointer, error.Code, error.Message)
		}
		fmt.Fprintf(stdout, "valid=%t; integrity=%t; rules=%t; files=%d; references=%d; external=%d; errors=%d\n", result.Valid, result.IntegrityValid, result.RulesValid, result.FilesChecked, result.ReferencesChecked, result.ExternalReferences, len(result.Errors))
		for _, rule := range result.Rules {
			fmt.Fprintf(stdout, "rule=%s; roots=%d; records=%d; transitions=%d; expectedStates=%d; excluded=%d; errors=%d\n", rule.ID, rule.Roots, rule.RecordsChecked, rule.TransitionsChecked, rule.ExpectedStates, rule.OutOfScopeRecords, rule.Errors)
		}
		for _, kind := range result.RecordTypes {
			fmt.Fprintf(stdout, "type=%s; records=%d; validation=%s; rule=%s\n", kind.Type, kind.Records, kind.Validation, kind.Rule)
		}
		fmt.Fprintf(stdout, "coverage: schemaFiles=%d; layoutFiles=%d; unboundModelTypes=%d; unitBindings=%d\n", result.Coverage.FilesWithSchema, result.Coverage.LayoutFilesChecked, len(result.Coverage.UnboundModelTypes), result.Coverage.UnitBindingsChecked)
		fmt.Fprintf(stdout, "models: bound=%d; unbound=%d; structuralContracts=%d; canonicalSchemas=%d\n", result.Coverage.BoundModelInstances, result.Coverage.UnboundModelInstances, result.Coverage.StructuralContractModelInstances, result.Coverage.CanonicalSchemaModelInstances)
	} else {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	return status
}
