package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path"
	"sort"
	"time"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
	"github.com/jedib0t/go-pretty/v6/table"
	regression "github.com/sajari/regression"
	progressbar "github.com/schollz/progressbar/v3"
)

var (
	version      = "1.0"
	useSimulator = flag.Bool("simulator", false, "use the TPM simulator")
	sortBy       = flag.String("sort_by", "keygen", "one of: [keygen, signing, size, name]")
)

func main() {
	if err := mainErr(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func mainErr() error {
	flag.Parse()

	var ordering estimateOrdering
	switch *sortBy {
	case "name":
		ordering = byName
	case "keygen":
		ordering = byKeygenWork
	case "signing":
		ordering = bySigWork
	case "size":
		ordering = bySigSize
	default:
		return fmt.Errorf("unknown sort_by value: '%v' (options are keygen, signing, size, name)", *sortBy)
	}

	tpm, err := getTPM(*useSimulator)
	if err != nil {
		return fmt.Errorf("could not open TPM: %v", err)
	}
	defer tpm.Close()

	info, err := getTPMInfo(tpm)
	if err != nil {
		return fmt.Errorf("could not get TPM info: %v", err)
	}

	hps, err := getHashPerformance(tpm)
	if err != nil {
		return fmt.Errorf("could not get SHA256 performance: %v", err)
	}
	printEstimates(*info, hps, ordering)
	return nil
}

func getTPM(sim bool) (transport.TPMCloser, error) {
	return transport.OpenTPM()
}

type tpmInfo struct {
	manufacturer string
	model        string
	fwVersion    string
	specVersion  string
}

func (info tpmInfo) String() string {
	return fmt.Sprintf("%v %v: TPM 2.0 rev %v (firmware %v)\n", info.manufacturer, info.model, info.specVersion, info.fwVersion)
}

func getCap(tpm transport.TPM, property tpm2.TPMPT) ([]byte, error) {
	getCap := tpm2.GetCapability{
		Capability:    tpm2.TPMCapTPMProperties,
		Property:      uint32(property),
		PropertyCount: 1,
	}
	cap, err := getCap.Execute(tpm)
	if err != nil {
		return nil, err
	}
	props, err := cap.CapabilityData.Data.TPMProperties().CheckUnwrap()
	if err != nil {
		return nil, err
	}
	if len(props.TPMProperty) != 1 {
		return nil, fmt.Errorf("TPM did not have property %v", property)
	}
	result := make([]byte, 4)
	binary.BigEndian.PutUint32(result, props.TPMProperty[0].Value)
	return result, nil
}

func getTPMInfo(tpm transport.TPM) (*tpmInfo, error) {
	mfr, err := getCap(tpm, tpm2.TPMPTManufacturer)
	if err != nil {
		return nil, err
	}
	fw1, err := getCap(tpm, tpm2.TPMPTFirmwareVersion1)
	if err != nil {
		return nil, err
	}
	fw2, err := getCap(tpm, tpm2.TPMPTFirmwareVersion2)
	if err != nil {
		return nil, err
	}
	specVersion, err := getCap(tpm, tpm2.TPMPTRevision)
	if err != nil {
		return nil, err
	}
	revision := binary.BigEndian.Uint32(specVersion)
	model1, err := getCap(tpm, tpm2.TPMPTVendorString1)
	if err != nil {
		return nil, err
	}
	model2, err := getCap(tpm, tpm2.TPMPTVendorString2)
	if err != nil {
		return nil, err
	}
	model3, err := getCap(tpm, tpm2.TPMPTVendorString3)
	if err != nil {
		return nil, err
	}
	model4, err := getCap(tpm, tpm2.TPMPTVendorString4)
	if err != nil {
		return nil, err
	}
	model := make([]byte, 0, 16)
	model = append(model, model1...)
	model = append(model, model2...)
	model = append(model, model3...)
	model = append(model, model4...)

	return &tpmInfo{
		manufacturer: fmt.Sprintf("%s", string(mfr)),
		fwVersion:    fmt.Sprintf("%0x%0x", fw1, fw2),
		specVersion:  fmt.Sprintf("%v.%v", revision/100, revision%100),
		model:        string(model),
	}, nil
}

func hash(tpm transport.TPM, count int) (*int64, error) {
	if count < 0 || count > 16 {
		return nil, fmt.Errorf("invalid count: %v (must be between 0 and 16)", count)
	}
	data := make([]byte, 64*count)

	hash := tpm2.Hash{
		HashAlg: tpm2.TPMAlgSHA256,
		Data: tpm2.TPM2BMaxBuffer{
			Buffer: data,
		},
	}
	start := time.Now()
	_, err := hash.Execute(tpm)
	duration := time.Now().Sub(start)
	if err != nil {
		return nil, fmt.Errorf("could not call TPM2_Hash: %v", err)
	}
	result := duration.Microseconds()
	return &result, nil
}

func getHashPerformance(tpm transport.TPM) (float64, error) {
	results := make([]float64, 17)
	count := 10
	// sum_0_to_16 = 136
	bar := progressbar.New(136 * count)
	defer bar.Close()
	for hashBlocks := range results {
		for j := 0; j < count; j++ {
			thisIteration, err := hash(tpm, hashBlocks)
			if err != nil {
				return 0, err
			}
			results[hashBlocks] += float64(*thisIteration)
			bar.Add(hashBlocks)
		}
		results[hashBlocks] /= float64(count)
	}
	bar.Finish()
	fmt.Printf("\n")

	var r regression.Regression
	for hashBlocks, avgTime := range results {
		r.Train(regression.DataPoint(avgTime, []float64{float64(hashBlocks)}))
	}
	r.Run()
	return 1000000000 / r.Coeff(1), nil
}

type estimateOrdering int

const (
	byName estimateOrdering = iota
	bySigSize
	bySigWork
	byKeygenWork
)

func printEstimates(info tpmInfo, hps float64, sortBy estimateOrdering) {
	algs := nistApprovedParameters
	sort.Slice(algs, func(i, j int) bool {
		switch sortBy {
		case byName:
			if algs[i].FriendlyName == algs[j].FriendlyName {
				return algs[i].W < algs[i].W
			}
			return algs[i].FriendlyName < algs[j].FriendlyName
		case bySigSize:
			return algs[i].SigSize < algs[j].SigSize
		case bySigWork:
			return algs[i].SigWork < algs[j].SigWork
		case byKeygenWork:
			return algs[i].KeygenWork < algs[j].KeygenWork
		}
		return false
	})

	tw := table.NewWriter()
	tw.AppendHeader(table.Row{
		"Friendly Name", "W (bits)", "Signatures", "Sig Size", "Est. Keygen", "Est. Signing",
	})
	for _, alg := range algs {
		keygen := time.Duration(float64(alg.KeygenWork) / hps * 1000000000)
		signing := time.Duration(float64(alg.SigWork) / hps * 1000000000)
		tw.AppendRow(table.Row{
			alg.FriendlyName,
			alg.W,
			1 << alg.H,
			alg.SigSize,
			keygen,
			signing,
		})
	}
	table := tw.Render()

	fmt.Printf("tpmhbs version %v\n", version)
	fmt.Printf("%v\n", info)
	fmt.Printf("Estimated (SHA256) hashes per second: %v\n", hps)
	fmt.Println(table)
	csv := tw.RenderCSV()
	wd, _ := os.Getwd()
	filename := path.Join(wd, fmt.Sprintf("tpmhbs.%v.%v.%v.csv", version, info.manufacturer, info.model))
	if err := os.WriteFile(filename, []byte(csv), 0666); err != nil {
		fmt.Fprintf(os.Stderr, "Could not write CSV file to %v.\n", filename)
	} else {
		fmt.Printf("Wrote CSV data to %v.\n", filename)
	}
}
