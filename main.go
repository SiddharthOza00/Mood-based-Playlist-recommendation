package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type Mood struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Playlist struct {
	ID    string   `json:"id"`
	Mood  string   `json:"mood"`
	Songs []string `json:"songs`
}

var (
	moods             = map[string]Mood{}
	playlists         = map[string]Playlist{}
	idCounter         = 1
	counterMux        sync.Mutex
	moodIDCounter     int
	playlistIDCounter int
)

const (
	SpotifyTokenURL = "https://accounts.spotify.com/api/token"
	ClientID        = "b558ca9bfd474472b9bb76cb8523c838"
	ClientSecret    = "e3fd214b727f450ba6a73b9d5fe1bb9f"
)

func main() {
	moods["1"] = Mood{ID: "1", Name: "happy"}
	moods["2"] = Mood{ID: "2", Name: "sad"}

	playlists["1"] = Playlist{ID: "1", Mood: "happy", Songs: []string{"Song 1", "Song 2"}}
	playlists["2"] = Playlist{ID: "2", Mood: "sad", Songs: []string{"Song 3"}}

	http.HandleFunc("/moods", chooseMoods)
	http.HandleFunc("/moods/", chooseMoodsByID)
	http.HandleFunc("/playlists/", choosePlaylists)

	log.Println("Server starting on port 8080....")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func chooseMoods(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getMoods(w, r)
	case http.MethodPost:
		addMood(w, r)
	case http.MethodPut:
		updateMood(w, r)
	case http.MethodDelete:
		deleteMood(w, r)
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, PUT, POST, DELETE, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func chooseMoodsByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getMoodsByID(w, r)
	case http.MethodPut:
		updateMood(w, r)
	case http.MethodDelete:
		deleteMood(w, r)
	default:
		http.Error(w, "Method not Allowed", http.StatusMethodNotAllowed)
	}
}

func getMoodsByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := strings.TrimPrefix(r.URL.Path, "/moods/")
	mood, exists := moods[id]
	if !exists {
		http.Error(w, "Mood not found", http.StatusNotFound)
		return
	}

	moodJSON, err := json.MarshalIndent(mood, "", " ")
	if err != nil {
		http.Error(w, "Failed to retrieve mood", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(moodJSON)
}

func choosePlaylists(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getPlaylist(w, r)
	case http.MethodPost:
		addPlaylist(w, r)
	case http.MethodPatch:
		updatePlaylist(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getSpotifyToken() (string, error) {
	authHeader := "Basic " + encodeClientCredentials(ClientID, ClientSecret)

	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", SpotifyTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	token, exists := result["access_token"].(string)
	if !exists {
		return "", fmt.Errorf("failed to retrieve access token")
	}

	return token, nil
}

// Fetch playlists from Spotify using the token
func fetchSpotifyPlaylists(mood string, token string) ([]string, error) {
	url := fmt.Sprintf("https://api.spotify.com/v1/search?q=mood:%s&type=playlist", mood)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var songs []string
	if playlistsObj, exists := result["playlists"].(map[string]interface{}); exists {
		if items, ok := playlistsObj["items"].([]interface{}); ok {
			for _, item := range items {
				if playlist, ok := item.(map[string]interface{}); ok {
					if tracks, ok := playlist["name"].(string); ok {
						songs = append(songs, tracks)
					}
				}
			}
		}
	}
	return songs, nil
}

func getMoods(w http.ResponseWriter, r *http.Request) {

	// moods := map[string][]string{
	// 	"happy":     {"Song 1", "Song 2", "Song 3"},
	// 	"sad":       {"Song 4", "Song 5"},
	// 	"energetic": {"Song 6", "Song 7"},
	// }

	w.Header().Set("Content-Type", "application/json")

	moodsJSON, err := json.Marshal(moods)
	if err != nil {
		http.Error(w, "Failed to retrieve moods", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(moodsJSON)
}

func addMood(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var newMood Mood

	err := json.NewDecoder(r.Body).Decode(&newMood)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	moodIDCounter++
	newMood.ID = generateMoodID()
	moods[newMood.ID] = newMood

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newMood)
}

func updateMood(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := strings.TrimPrefix(r.URL.Path, "/moods/")

	_, exists := moods[id]
	if !exists {
		http.Error(w, "Mood not found", http.StatusNotFound)
		return
	}

	var updatedMood Mood
	err := json.NewDecoder(r.Body).Decode(&updatedMood)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	updatedMood.ID = id
	moods[id] = updatedMood

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updatedMood)
}

func deleteMood(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/moods/")

	if _, exists := moods[id]; !exists {
		http.Error(w, "Mood not enough", http.StatusNotFound)
		return
	}

	delete(moods, id)
	w.WriteHeader(http.StatusNoContent)
}

func handleMoodsOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET, POST, PUT, DELETE, OPTIONS")
	w.WriteHeader(http.StatusOK)
}

func getPlaylist(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	mood := strings.TrimPrefix(r.URL.Path, "/playlists/")
	// token, err := getSpotifyToken()
	// if err != nil {
	// 	http.Error(w, "Failed to get Spotify token", http.StatusInternalServerError)
	// 	return
	// }

	// Getting Hardcoded playlists
	for _, playlist := range playlists {
		if playlist.Mood == mood {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(playlist)
			return
		}
	}
	http.Error(w, "Playlist Not Found", http.StatusNotFound)

	// songs, err := fetchSpotifyPlaylists(mood, token)
	// if err != nil {
	// 	http.Error(w, "Failed to fetch playlists", http.StatusInternalServerError)
	// 	return
	// }

	// playlist := Playlist{Mood: mood, Songs: songs}
	// playlists[mood] = playlist

	// w.WriteHeader(http.StatusOK)
	// json.NewEncoder(w).Encode(playlist)
}

func addPlaylist(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var newPlaylist Playlist
	err := json.NewDecoder(r.Body).Decode(&newPlaylist)
	if err != nil {
		http.Error(w, "Invalid Input", http.StatusBadRequest)
		return
	}

	// if _, exists := moods[newPlaylist.Mood]; !exists {
	// 	http.Error(w, "Mood not found", http.StatusBadRequest)
	// 	return
	// }

	playlistIDCounter++
	newPlaylist.ID = generatePlaylistID()
	playlists[newPlaylist.ID] = newPlaylist
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newPlaylist)
}

func updatePlaylist(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	mood := strings.TrimPrefix(r.URL.Path, "/playlists/")

	playlist, exists := playlists[mood]
	if !exists {
		http.Error(w, "Playlist not found", http.StatusNotFound)
		return
	}

	var updates struct {
		AddSongs    []string `json:"add_songs"`
		RemoveSongs []string `json:"remove_songs"`
	}
	err := json.NewDecoder(r.Body).Decode(&updates)
	if err != nil {
		http.Error(w, "Invalid Input", http.StatusBadRequest)
		return
	}

	playlist.Songs = append(playlist.Songs, updates.AddSongs...)
	for _, songToRemove := range updates.RemoveSongs {
		playlist.Songs = removeSong(playlist.Songs, songToRemove)
	}

	playlists[mood] = playlist
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(playlist)
}

func generateID() string {
	return uuid.New().String()
}

func removeSong(songs []string, songToRemove string) []string {
	updatedSongs := []string{}

	for _, song := range songs {
		if song != songToRemove {
			updatedSongs = append(updatedSongs, song)
		}
	}

	return updatedSongs
}

func encodeClientCredentials(clientID, clientSecret string) string {
	credentials := clientID + ":" + clientSecret
	return base64.StdEncoding.EncodeToString([]byte(credentials))
}

func generateMoodID() string {
	return fmt.Sprintf("%d", moodIDCounter)
}

func generatePlaylistID() string {
	return fmt.Sprintf("%d", playlistIDCounter)
}
