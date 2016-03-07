package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"

	"github.com/kellydunn/golang-geo"
	"github.com/paulmach/go.geojson"
)

type (
	settings struct {
		http string
		a    string
		b    string
	}

	job struct {
		json []byte
		err  error
	}

	ErrorMessage struct {
		geojson.FeatureCollection
		Error   string `json:"error,omitempty"`
		Message string `json:"message,omitempty"`
	}

	point struct {
		x float64
		y float64
	}

	bounds struct {
		min point
		max point
	}
)

var (
	fileAreas, filePoints string
)

func init() {
	flag.StringVar(&fileAreas, "areas", "", "area geojson file")
	flag.StringVar(&filePoints, "points", "", "points geojson file")
}

func main() {
	flag.Parse()

	if fileAreas == "" || filePoints == "" {
		fmt.Println("expecting both `areas` and `points`")
		flag.Usage()
		os.Exit(1)
	}

	areas, err := ioutil.ReadFile(fileAreas)
	if err != nil {
		fmt.Printf("error reading %s, error: %v\n", fileAreas, err)
		os.Exit(1)
	}

	points, err := ioutil.ReadFile(filePoints)
	if err != nil {
		fmt.Printf("error reading %s, error: %v\n", filePoints, err)
		os.Exit(1)
	}

	result, err := reportGenerator(areas, points)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(string(result))
}

func reportGenerator(bigbuddy, lilbuddy []byte) ([]byte, error) {
	boundaries, err := geojson.UnmarshalFeatureCollection(bigbuddy)
	if err != nil {
		log.Printf("error unmarshaling area file, error %v\n", err)
		return []byte{}, err
	}

	points, err := geojson.UnmarshalFeatureCollection(lilbuddy)
	if err != nil {
		log.Printf("error unmarshaling points file, error %v\n", err)
		return []byte{}, err
	}

	definition := buildDefinition(boundaries.Features)
	aggregate(boundaries, points.Features, definition)

	result, err := boundaries.MarshalJSON()
	if err != nil {
		log.Printf("error producing geojson, error %v\n", err)
		return []byte{}, err
	}
	return result, nil
}

func totalSum(ls []int) int {
	sum := 0
	for _, l := range ls {
		sum += l
	}
	return sum
}

func buildBounds(features []*geojson.Feature) map[int]*bounds {
	results := make(map[int]*bounds)
	for pid, feature := range features {
		bounds := newBounds()
		for _, mpg := range feature.Geometry.MultiPolygon {
			for _, pg := range mpg {
				for _, pt := range pg {
					p := point{pt[0], pt[1]}
					bounds.extend(p)
				}
			}
		}
		results[pid] = bounds
	}
	return results
}

func buildDefinition(features []*geojson.Feature) map[int][][]*geo.Polygon {
	definition := make(map[int][][]*geo.Polygon)
	for fid, feature := range features {
		group := make([][]*geo.Polygon, 0)
		for _, mpg := range feature.Geometry.MultiPolygon {
			polygongroup := make([]*geo.Polygon, 0)
			for _, pg := range mpg {
				pointGroups := make([]*geo.Point, 0)
				for _, pt := range pg {
					lat := pt[1]
					lng := pt[0]
					point := geo.NewPoint(lat, lng)
					pointGroups = append(pointGroups, point)
				}
				polygon := geo.NewPolygon(pointGroups)
				polygongroup = append(polygongroup, polygon)
			}
			group = append(group, polygongroup)
		}
		definition[fid] = group
	}
	return definition
}

func aggregate(collection *geojson.FeatureCollection, features []*geojson.Feature, definition map[int][][]*geo.Polygon) {
	log.Printf("features count (%v)\n", len(features))
	for _, pt := range features {
		lat := pt.Geometry.Point[1]
		lng := pt.Geometry.Point[0]
		point := geo.NewPoint(lat, lng)
		for pid, polygroup := range definition {
			polycount := len(polygroup)
			for _, polygons := range polygroup {
				sum := make([]int, 0)
				containCount := 0
				for _, polygon := range polygons {
					if polygon.Contains(point) {
						containCount++
					}
				}
				// TODO Remove sum
				// Sum is redundant, containCount is enough
				if containCount%2 == 1 {
					sum = append(sum, 1)
				}

				total := totalSum(sum)
				var count int
				count, ok := collection.Features[pid].Properties["count"].(int)
				if !ok {
					count = 0
					collection.Features[pid].Properties["count"] = 0
				}

				if total > 0 {
					if total == 1 {
						collection.Features[pid].Properties["count"] = count + 1
						continue
					} else {
						log.Print(total)
						if polycount > 1 {
							collection.Features[pid].Properties["count"] = count + 1
							continue
						}
					}
				}

				_, ok = collection.Features[pid].Properties["total"]
				if !ok {
					collection.Features[pid].Properties["total"] = len(features)
				}
			}
		}
	}
}

func newBounds() *bounds {
	return &bounds{
		point{math.Inf(1), math.Inf(1)},
		point{math.Inf(-1), math.Inf(-1)},
	}
}

func (b *bounds) extend(p point) {
	b.min.x = math.Min(b.min.x, p.x)
	b.min.y = math.Min(b.min.y, p.y)
	b.max.x = math.Max(b.max.x, p.x)
	b.max.y = math.Max(b.max.y, p.y)
}

func overlaps(a, b *bounds) bool {
	return a.min.x <= b.max.x && a.min.y <= b.max.y && a.max.x >= b.min.x && a.max.y >= b.min.y
}
