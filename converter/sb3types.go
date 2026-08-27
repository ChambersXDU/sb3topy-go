package converter

import "encoding/json"

// ProjectJSON represents Scratch 3 project.json structure
type ProjectJSON struct {
	Targets []TargetJSON `json:"targets"`
}

// TargetJSON represents a Target (Stage or Sprite) in project.json
type TargetJSON struct {
	IsStage        bool                   `json:"isStage"`
	Name           string                 `json:"name"`
	Variables      map[string]interface{} `json:"variables"`
	Lists          map[string]interface{} `json:"lists"`
	Broadcasts     map[string]string      `json:"broadcasts"`
	Blocks         map[string]interface{} `json:"blocks"`
	Costumes       []CostumeJSON          `json:"costumes"`
	Sounds         []SoundJSON            `json:"sounds"`
	CurrentCostume int                    `json:"currentCostume"`
	LayerOrder     int                    `json:"layerOrder"`
	Volume         *float64               `json:"volume"`
	X              float64                `json:"x"`
	Y              float64                `json:"y"`
	Size           *float64               `json:"size"`
	Direction      float64                `json:"direction"`
	Visible        bool                   `json:"visible"`
	RotationStyle  string                 `json:"rotationStyle"`
}

// CostumeJSON represents costume data
type CostumeJSON struct {
	Name             string  `json:"name"`
	AssetID          string  `json:"assetId"`
	DataFormat       string  `json:"dataFormat"`
	Md5Ext           string  `json:"md5ext"`
	BitmapResolution float64 `json:"bitmapResolution"`
	CenterX          float64 `json:"rotationCenterX"`
	CenterY          float64 `json:"rotationCenterY"`
}

// SoundJSON represents sound data
type SoundJSON struct {
	Name       string `json:"name"`
	AssetID    string `json:"assetId"`
	DataFormat string `json:"dataFormat"`
	Md5Ext     string `json:"md5ext"`
	Rate       int    `json:"rate"`
	SampleCnt  int    `json:"sampleCount"`
}

// RawBlockData represents a Scratch block object
type RawBlockData struct {
	Opcode   string                 `json:"opcode"`
	Next     interface{}            `json:"next"`
	Parent   interface{}            `json:"parent"`
	Inputs   map[string]interface{} `json:"inputs"`
	Fields   map[string]interface{} `json:"fields"`
	Shadow   bool                   `json:"shadow"`
	TopLevel bool                   `json:"topLevel"`
	Mutation map[string]interface{} `json:"mutation"`
}

// ParseRawBlock parses a block interface{} from project.json
func ParseRawBlock(raw interface{}) (*RawBlockData, bool) {
	bytes, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}
	var b RawBlockData
	if err := json.Unmarshal(bytes, &b); err != nil || b.Opcode == "" {
		return nil, false
	}
	return &b, true
}
