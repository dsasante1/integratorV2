package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
	"log/slog"
	"slices"
)

type ServiceAccountCredentials struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
	UniverseDomain          string `json:"universe_domain"`
}

type FirebaseConfig struct {
	ProjectID    string
	DatabaseURL  string
	Credentials  ServiceAccountCredentials
}

type FirebaseClient struct {
	App       *firebase.App
	Firestore *firestore.Client
}

var FirebaseConnection *FirebaseClient

func NewFirebaseClient(config *FirebaseConfig) (*FirebaseClient, error) {
	ctx := context.Background()

	credentialsJSON, err := json.Marshal(config.Credentials)
	if err != nil {
		slog.Error("Failed to marshal Firebase credentials", slog.Any("error", err))
		return nil, err
	}

	opt := option.WithCredentialsJSON(credentialsJSON)

	firebaseConfig := &firebase.Config{
		ProjectID:   config.ProjectID,
		DatabaseURL: config.DatabaseURL,
	}

	app, err := firebase.NewApp(ctx, firebaseConfig, opt)
	if err != nil {
		slog.Error("Failed to create Firebase app", slog.Any("error", err))
		return nil, err
	}

	firestoreClient, err := app.Firestore(ctx)
	if err != nil {
		slog.Error("Failed to create Firestore client", slog.Any("error", err))
		return nil, err
	}

	return &FirebaseClient{
		App:       app,
		Firestore: firestoreClient,
	}, nil
}


func validateEnvVariables(envVariables []string) error {
	if slices.Contains(envVariables, "") {
			return errors.New("missing required Firebase config environment variables")
		}
	return nil
}


func LoadFirebaseConfig() (*FirebaseConfig, error) {
	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	databaseURL := os.Getenv("FIREBASE_DATABASE_URL")
	firebaseType := os.Getenv("FIREBASE_TYPE")
	privateKeyID := os.Getenv("FIREBASE_PRIVATE_KEY_ID")
	privateKey := os.Getenv("FIREBASE_PRIVATE_KEY")
	clientEmail := os.Getenv("FIREBASE_CLIENT_EMAIL")
	clientID := os.Getenv("FIREBASE_CLIENT_ID")
	authURI := os.Getenv("FIREBASE_AUTH_URI")
	tokenURI := os.Getenv("FIREBASE_TOKEN_URI")
	authProviderCertURL := os.Getenv("FIREBASE_AUTH_PROVIDER_X509_CERT_URL")
	clientCertURL := os.Getenv("FIREBASE_CLIENT_X509_CERT_URL")
	universeDomain := os.Getenv("FIREBASE_UNIVERSE_DOMAIN")

	requiredVars := []string{
		projectID,
		databaseURL,
		firebaseType,
		privateKeyID,
		privateKey,
		clientEmail,
		clientID,
		authURI,
		tokenURI,
		authProviderCertURL,
		clientCertURL,
		universeDomain,
	}

	if err := validateEnvVariables(requiredVars); err != nil {
		slog.Error("Environment variable validation failed", slog.Any("error", err))
		return nil, err
	}

	credentials := ServiceAccountCredentials{
		Type:                    firebaseType,
		ProjectID:               projectID,
		PrivateKeyID:            privateKeyID,
		PrivateKey:              privateKey,
		ClientEmail:             clientEmail,
		ClientID:                clientID,
		AuthURI:                 authURI,
		TokenURI:                tokenURI,
		AuthProviderX509CertURL: authProviderCertURL,
		ClientX509CertURL:       clientCertURL,
		UniverseDomain:          universeDomain,
	}

	return &FirebaseConfig{
		ProjectID:   projectID,
		DatabaseURL: databaseURL,
		Credentials: credentials,
	}, nil
}

func InitFireStore() error {
	slog.Info("Initializing Firebase connection from environment variables")
	
	firebaseConfig, err := LoadFirebaseConfig()
	if err != nil {
		slog.Error("Failed to load Firebase config from environment variables", slog.Any("error", err))
		return err
	}

	FirebaseConnection, err = NewFirebaseClient(firebaseConfig)
	if err != nil {
		slog.Error("Failed to initialize Firebase client", slog.Any("error", err))
		return err
	}

	slog.Info("Firebase connection initialized successfully")
	return nil
}

func CloseFirebaseConnection() error {
	if FirebaseConnection != nil && FirebaseConnection.Firestore != nil {
		err := FirebaseConnection.Firestore.Close()
		if err != nil {
			slog.Error("Failed to close Firebase connection", slog.Any("error", err))
			return err
		}
		slog.Info("Firebase connection closed successfully")
		FirebaseConnection = nil
	}
	return nil
}

func GetFirebaseClient() *FirebaseClient {
	if FirebaseConnection == nil {
		slog.Error("Firebase client not initialized. Call InitFireStore() first.")
		return nil
	}
	return FirebaseConnection
}
