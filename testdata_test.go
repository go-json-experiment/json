// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

type jsonTestdataEntry struct {
	name string
	data []byte
	new  func() interface{} // nil if there is no concrete type for this
}

var (
	jsonTestdataOnce sync.Once
	jsonTestdataLazy []jsonTestdataEntry
)

func jsonTestdata() []jsonTestdataEntry {
	jsonTestdataOnce.Do(func() {
		fis, err := ioutil.ReadDir("testdata")
		if err != nil {
			panic(err)
		}
		sort.Slice(fis, func(i, j int) bool { return fis[i].Name() < fis[j].Name() })
		for _, fi := range fis {
			if !strings.HasSuffix(fi.Name(), ".json.gz") {
				break
			}

			// Skip large files for a short test run.
			if testing.Short() {
				fi, err := os.Stat(filepath.Join("testdata", fi.Name()))
				if err == nil && fi.Size() > 1e5 {
					continue
				}
			}

			// Convert snake_case file name to CamelCase.
			words := strings.Split(strings.TrimSuffix(fi.Name(), ".json.gz"), "_")
			for i := range words {
				words[i] = strings.Title(words[i])
			}
			name := strings.Join(words, "")

			// Read and decompress the test data.
			b, err := ioutil.ReadFile(filepath.Join("testdata", fi.Name()))
			if err != nil {
				panic(err)
			}
			zr, err := gzip.NewReader(bytes.NewReader(b))
			if err != nil {
				panic(err)
			}
			data, err := ioutil.ReadAll(zr)
			if err != nil {
				panic(err)
			}

			// Check whether there is a concrete type for this data.
			var newFn func() interface{}
			switch name {
			case "GolangSource":
				newFn = func() interface{} { return new(golangRoot) }
			case "StringEscaped":
				newFn = func() interface{} { return new(stringRoot) }
			case "StringUnicode":
				newFn = func() interface{} { return new(stringRoot) }
			case "SyntheaFhir":
				newFn = func() interface{} { return new(syntheaRoot) }
			}

			jsonTestdataLazy = append(jsonTestdataLazy, jsonTestdataEntry{name, data, newFn})
		}
	})
	return jsonTestdataLazy
}

type (
	golangRoot struct {
		Tree     *golangNode `json:"tree"`
		Username string      `json:"username"`
	}
	golangNode struct {
		Name     string       `json:"name"`
		Kids     []golangNode `json:"kids"`
		CLWeight float64      `json:"cl_weight"`
		Touches  int          `json:"touches"`
		MinT     uint64       `json:"min_t"`
		MaxT     uint64       `json:"max_t"`
		MeanT    uint64       `json:"mean_t"`
	}
)

type (
	stringRoot struct {
		Arabic                             string `json:"Arabic"`
		ArabicPresentationFormsA           string `json:"Arabic Presentation Forms-A"`
		ArabicPresentationFormsB           string `json:"Arabic Presentation Forms-B"`
		Armenian                           string `json:"Armenian"`
		Arrows                             string `json:"Arrows"`
		Bengali                            string `json:"Bengali"`
		Bopomofo                           string `json:"Bopomofo"`
		BoxDrawing                         string `json:"Box Drawing"`
		CJKCompatibility                   string `json:"CJK Compatibility"`
		CJKCompatibilityForms              string `json:"CJK Compatibility Forms"`
		CJKCompatibilityIdeographs         string `json:"CJK Compatibility Ideographs"`
		CJKSymbolsAndPunctuation           string `json:"CJK Symbols and Punctuation"`
		CJKUnifiedIdeographs               string `json:"CJK Unified Ideographs"`
		CJKUnifiedIdeographsExtensionA     string `json:"CJK Unified Ideographs Extension A"`
		CJKUnifiedIdeographsExtensionB     string `json:"CJK Unified Ideographs Extension B"`
		Cherokee                           string `json:"Cherokee"`
		CurrencySymbols                    string `json:"Currency Symbols"`
		Cyrillic                           string `json:"Cyrillic"`
		CyrillicSupplementary              string `json:"Cyrillic Supplementary"`
		Devanagari                         string `json:"Devanagari"`
		EnclosedAlphanumerics              string `json:"Enclosed Alphanumerics"`
		EnclosedCJKLettersAndMonths        string `json:"Enclosed CJK Letters and Months"`
		Ethiopic                           string `json:"Ethiopic"`
		GeometricShapes                    string `json:"Geometric Shapes"`
		Georgian                           string `json:"Georgian"`
		GreekAndCoptic                     string `json:"Greek and Coptic"`
		Gujarati                           string `json:"Gujarati"`
		Gurmukhi                           string `json:"Gurmukhi"`
		HangulCompatibilityJamo            string `json:"Hangul Compatibility Jamo"`
		HangulJamo                         string `json:"Hangul Jamo"`
		HangulSyllables                    string `json:"Hangul Syllables"`
		Hebrew                             string `json:"Hebrew"`
		Hiragana                           string `json:"Hiragana"`
		IPAExtentions                      string `json:"IPA Extentions"`
		KangxiRadicals                     string `json:"Kangxi Radicals"`
		Katakana                           string `json:"Katakana"`
		Khmer                              string `json:"Khmer"`
		KhmerSymbols                       string `json:"Khmer Symbols"`
		Latin                              string `json:"Latin"`
		LatinExtendedAdditional            string `json:"Latin Extended Additional"`
		Latin1Supplement                   string `json:"Latin-1 Supplement"`
		LatinExtendedA                     string `json:"Latin-Extended A"`
		LatinExtendedB                     string `json:"Latin-Extended B"`
		LetterlikeSymbols                  string `json:"Letterlike Symbols"`
		Malayalam                          string `json:"Malayalam"`
		MathematicalAlphanumericSymbols    string `json:"Mathematical Alphanumeric Symbols"`
		MathematicalOperators              string `json:"Mathematical Operators"`
		MiscellaneousSymbols               string `json:"Miscellaneous Symbols"`
		Mongolian                          string `json:"Mongolian"`
		NumberForms                        string `json:"Number Forms"`
		Oriya                              string `json:"Oriya"`
		PhoneticExtensions                 string `json:"Phonetic Extensions"`
		SupplementalArrowsB                string `json:"Supplemental Arrows-B"`
		Syriac                             string `json:"Syriac"`
		Tamil                              string `json:"Tamil"`
		Thaana                             string `json:"Thaana"`
		Thai                               string `json:"Thai"`
		UnifiedCanadianAboriginalSyllabics string `json:"Unified Canadian Aboriginal Syllabics"`
		YiRadicals                         string `json:"Yi Radicals"`
		YiSyllables                        string `json:"Yi Syllables"`
	}
)

type (
	syntheaRoot struct {
		Entry []struct {
			FullURL string `json:"fullUrl"`
			Request *struct {
				Method string `json:"method"`
				URL    string `json:"url"`
			} `json:"request"`
			Resource *struct {
				AbatementDateTime time.Time   `json:"abatementDateTime"`
				AchievementStatus syntheaCode `json:"achievementStatus"`
				Active            bool        `json:"active"`
				Activity          []struct {
					Detail *struct {
						Code     syntheaCode      `json:"code"`
						Location syntheaReference `json:"location"`
						Status   string           `json:"status"`
					} `json:"detail"`
				} `json:"activity"`
				Address        []syntheaAddress   `json:"address"`
				Addresses      []syntheaReference `json:"addresses"`
				AuthoredOn     time.Time          `json:"authoredOn"`
				BillablePeriod syntheaRange       `json:"billablePeriod"`
				BirthDate      string             `json:"birthDate"`
				CareTeam       []struct {
					Provider  syntheaReference `json:"provider"`
					Reference string           `json:"reference"`
					Role      syntheaCode      `json:"role"`
					Sequence  int64            `json:"sequence"`
				} `json:"careTeam"`
				Category       []syntheaCode    `json:"category"`
				Claim          syntheaReference `json:"claim"`
				Class          syntheaCoding    `json:"class"`
				ClinicalStatus syntheaCode      `json:"clinicalStatus"`
				Code           syntheaCode      `json:"code"`
				Communication  []struct {
					Language syntheaCode `json:"language"`
				} `json:"communication"`
				Component []struct {
					Code          syntheaCode   `json:"code"`
					ValueQuantity syntheaCoding `json:"valueQuantity"`
				} `json:"component"`
				Contained []struct {
					Beneficiary  syntheaReference   `json:"beneficiary"`
					ID           string             `json:"id"`
					Intent       string             `json:"intent"`
					Payor        []syntheaReference `json:"payor"`
					Performer    []syntheaReference `json:"performer"`
					Requester    syntheaReference   `json:"requester"`
					ResourceType string             `json:"resourceType"`
					Status       string             `json:"status"`
					Subject      syntheaReference   `json:"subject"`
					Type         syntheaCode        `json:"type"`
				} `json:"contained"`
				Created          time.Time   `json:"created"`
				DeceasedDateTime time.Time   `json:"deceasedDateTime"`
				Description      syntheaCode `json:"description"`
				Diagnosis        []struct {
					DiagnosisReference syntheaReference `json:"diagnosisReference"`
					Sequence           int64            `json:"sequence"`
					Type               []syntheaCode    `json:"type"`
				} `json:"diagnosis"`
				DosageInstruction []struct {
					AsNeededBoolean bool `json:"asNeededBoolean"`
					DoseAndRate     []struct {
						DoseQuantity *struct {
							Value float64 `json:"value"`
						} `json:"doseQuantity"`
						Type syntheaCode `json:"type"`
					} `json:"doseAndRate"`
					Sequence int64 `json:"sequence"`
					Timing   *struct {
						Repeat *struct {
							Frequency  int64   `json:"frequency"`
							Period     float64 `json:"period"`
							PeriodUnit string  `json:"periodUnit"`
						} `json:"repeat"`
					} `json:"timing"`
				} `json:"dosageInstruction"`
				EffectiveDateTime time.Time          `json:"effectiveDateTime"`
				Encounter         syntheaReference   `json:"encounter"`
				Extension         []syntheaExtension `json:"extension"`
				Gender            string             `json:"gender"`
				Goal              []syntheaReference `json:"goal"`
				ID                string             `json:"id"`
				Identifier        []struct {
					System string      `json:"system"`
					Type   syntheaCode `json:"type"`
					Use    string      `json:"use"`
					Value  string      `json:"value"`
				} `json:"identifier"`
				Insurance []struct {
					Coverage syntheaReference `json:"coverage"`
					Focal    bool             `json:"focal"`
					Sequence int64            `json:"sequence"`
				} `json:"insurance"`
				Insurer syntheaReference `json:"insurer"`
				Intent  string           `json:"intent"`
				Issued  time.Time        `json:"issued"`
				Item    []struct {
					Adjudication []struct {
						Amount   syntheaCurrency `json:"amount"`
						Category syntheaCode     `json:"category"`
					} `json:"adjudication"`
					Category                syntheaCode        `json:"category"`
					DiagnosisSequence       []int64            `json:"diagnosisSequence"`
					Encounter               []syntheaReference `json:"encounter"`
					InformationSequence     []int64            `json:"informationSequence"`
					LocationCodeableConcept syntheaCode        `json:"locationCodeableConcept"`
					Net                     syntheaCurrency    `json:"net"`
					ProcedureSequence       []int64            `json:"procedureSequence"`
					ProductOrService        syntheaCode        `json:"productOrService"`
					Sequence                int64              `json:"sequence"`
					ServicedPeriod          syntheaRange       `json:"servicedPeriod"`
				} `json:"item"`
				LifecycleStatus           string             `json:"lifecycleStatus"`
				ManagingOrganization      []syntheaReference `json:"managingOrganization"`
				MaritalStatus             syntheaCode        `json:"maritalStatus"`
				MedicationCodeableConcept syntheaCode        `json:"medicationCodeableConcept"`
				MultipleBirthBoolean      bool               `json:"multipleBirthBoolean"`
				Name                      RawValue           `json:"name"`
				NumberOfInstances         int64              `json:"numberOfInstances"`
				NumberOfSeries            int64              `json:"numberOfSeries"`
				OccurrenceDateTime        time.Time          `json:"occurrenceDateTime"`
				OnsetDateTime             time.Time          `json:"onsetDateTime"`
				Outcome                   string             `json:"outcome"`
				Participant               []struct {
					Individual syntheaReference `json:"individual"`
					Member     syntheaReference `json:"member"`
					Role       []syntheaCode    `json:"role"`
				} `json:"participant"`
				Patient syntheaReference `json:"patient"`
				Payment *struct {
					Amount syntheaCurrency `json:"amount"`
				} `json:"payment"`
				PerformedPeriod syntheaRange     `json:"performedPeriod"`
				Period          syntheaRange     `json:"period"`
				Prescription    syntheaReference `json:"prescription"`
				PrimarySource   bool             `json:"primarySource"`
				Priority        syntheaCode      `json:"priority"`
				Procedure       []struct {
					ProcedureReference syntheaReference `json:"procedureReference"`
					Sequence           int64            `json:"sequence"`
				} `json:"procedure"`
				Provider        syntheaReference   `json:"provider"`
				ReasonCode      []syntheaCode      `json:"reasonCode"`
				ReasonReference []syntheaReference `json:"reasonReference"`
				RecordedDate    time.Time          `json:"recordedDate"`
				Referral        syntheaReference   `json:"referral"`
				Requester       syntheaReference   `json:"requester"`
				ResourceType    string             `json:"resourceType"`
				Result          []syntheaReference `json:"result"`
				Series          []struct {
					BodySite syntheaCoding `json:"bodySite"`
					Instance []struct {
						Number   int64         `json:"number"`
						SopClass syntheaCoding `json:"sopClass"`
						Title    string        `json:"title"`
						UID      string        `json:"uid"`
					} `json:"instance"`
					Modality          syntheaCoding `json:"modality"`
					Number            int64         `json:"number"`
					NumberOfInstances int64         `json:"numberOfInstances"`
					Started           string        `json:"started"`
					UID               string        `json:"uid"`
				} `json:"series"`
				ServiceProvider syntheaReference `json:"serviceProvider"`
				Started         time.Time        `json:"started"`
				Status          string           `json:"status"`
				Subject         syntheaReference `json:"subject"`
				SupportingInfo  []struct {
					Category       syntheaCode      `json:"category"`
					Sequence       int64            `json:"sequence"`
					ValueReference syntheaReference `json:"valueReference"`
				} `json:"supportingInfo"`
				Telecom              []map[string]string `json:"telecom"`
				Text                 map[string]string   `json:"text"`
				Total                RawValue            `json:"total"`
				Type                 RawValue            `json:"type"`
				Use                  string              `json:"use"`
				VaccineCode          syntheaCode         `json:"vaccineCode"`
				ValueCodeableConcept syntheaCode         `json:"valueCodeableConcept"`
				ValueQuantity        syntheaCoding       `json:"valueQuantity"`
				VerificationStatus   syntheaCode         `json:"verificationStatus"`
			} `json:"resource"`
		} `json:"entry"`
		ResourceType string `json:"resourceType"`
		Type         string `json:"type"`
	}
	syntheaCode struct {
		Coding []syntheaCoding `json:"coding"`
		Text   string          `json:"text"`
	}
	syntheaCoding struct {
		Code    string  `json:"code"`
		Display string  `json:"display"`
		System  string  `json:"system"`
		Unit    string  `json:"unit"`
		Value   float64 `json:"value"`
	}
	syntheaReference struct {
		Display   string `json:"display"`
		Reference string `json:"reference"`
	}
	syntheaAddress struct {
		City       string             `json:"city"`
		Country    string             `json:"country"`
		Extension  []syntheaExtension `json:"extension"`
		Line       []string           `json:"line"`
		PostalCode string             `json:"postalCode"`
		State      string             `json:"state"`
	}
	syntheaExtension struct {
		URL          string             `json:"url"`
		ValueAddress syntheaAddress     `json:"valueAddress"`
		ValueCode    string             `json:"valueCode"`
		ValueDecimal float64            `json:"valueDecimal"`
		ValueString  string             `json:"valueString"`
		Extension    []syntheaExtension `json:"extension"`
	}
	syntheaRange struct {
		End   time.Time `json:"end"`
		Start time.Time `json:"start"`
	}
	syntheaCurrency struct {
		Currency string  `json:"currency"`
		Value    float64 `json:"value"`
	}
)
