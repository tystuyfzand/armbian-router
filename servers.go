package main

import (
	log "github.com/sirupsen/logrus"
	"math"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	checkClient = &http.Client{
		Timeout: 10 * time.Second,
	}
)

type Server struct {
	Available bool
	Host      string
	Path      string
	Latitude  float64
	Longitude float64
}

type ServerList []*Server

func (s ServerList) checkLoop() {
	t := time.NewTicker(60 * time.Second)

	for {
		<-t.C

		s.Check()
	}
}

// Check will request the index from all servers
// If a server does not respond in 10 seconds, it is considered offline.
// This will wait until all checks are complete.
func (s ServerList) Check() {
	var wg sync.WaitGroup

	for _, server := range s {
		wg.Add(1)

		go func(server *Server) {
			req, err := http.NewRequest(http.MethodGet, "https://"+server.Host+"/"+strings.TrimLeft(server.Path, "/"), nil)

			req.Header.Set("User-Agent", "ArmbianRouter/1.0 (Go "+runtime.Version()+")")

			if err != nil {
				// This should never happen.
				log.WithError(err).Warning("Invalid request! This should not happen, please check config.")
				return
			}

			res, err := checkClient.Do(req)

			if err != nil {
				log.WithField("server", server.Host).Info("Server went offline")
				server.Available = false
				return
			}

			if (res.StatusCode == http.StatusOK || res.StatusCode == http.StatusMovedPermanently || res.StatusCode == http.StatusFound) &&
				!server.Available {
				server.Available = true
				log.WithField("server", server.Host).Info("Server is online")
			}
			wg.Done()
		}(server)
	}

	wg.Wait()
}

// Closest will use GeoIP on the IP provided and find the closest server.
// Return values are the closest server, the distance, and if an error occurred.
func (s ServerList) Closest(ip net.IP) (*Server, float64, error) {
	var city City
	err := db.Lookup(ip, &city)

	if err != nil {
		return nil, -1, err
	}

	var closest *Server
	var closestDistance float64 = -1

	for _, server := range s {
		if !server.Available {
			continue
		}

		distance := Distance(city.Location.Latitude, city.Location.Longitude, server.Latitude, server.Longitude)

		if closestDistance == -1 || distance < closestDistance {
			closestDistance = distance
			closest = server
		}
	}

	return closest, closestDistance, nil
}

// haversin(θ) function
func hsin(theta float64) float64 {
	return math.Pow(math.Sin(theta/2), 2)
}

// Distance function returns the distance (in meters) between two points of
//     a given longitude and latitude relatively accurately (using a spherical
//     approximation of the Earth) through the Haversin Distance Formula for
//     great arc distance on a sphere with accuracy for small distances
//
// point coordinates are supplied in degrees and converted into rad. in the func
//
// distance returned is METERS!!!!!!
// http://en.wikipedia.org/wiki/Haversine_formula
func Distance(lat1, lon1, lat2, lon2 float64) float64 {
	// convert to radians
	// must cast radius as float to multiply later
	var la1, lo1, la2, lo2, r float64
	la1 = lat1 * math.Pi / 180
	lo1 = lon1 * math.Pi / 180
	la2 = lat2 * math.Pi / 180
	lo2 = lon2 * math.Pi / 180

	r = 6378100 // Earth radius in METERS

	// calculate
	h := hsin(la2-la1) + math.Cos(la1)*math.Cos(la2)*hsin(lo2-lo1)

	return 2 * r * math.Asin(math.Sqrt(h))
}
