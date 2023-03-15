/*
 * Copyright 2023 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package security

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	authEndpoint     string
	authClientId     string
	authClientSecret string

	token *OpenidToken
}

func NewClient(authEndpoint string, authClientId string, authClientSecret string) (c *Client, err error) {
	c = &Client{
		authEndpoint:     authEndpoint,
		authClientId:     authClientId,
		authClientSecret: authClientSecret,
	}
	_, err = c.GetToken()
	if err != nil {
		return nil, err
	}
	return c, err
}

func (c *Client) GetToken() (token OpenidToken, err error) {
	if c.token != nil && c.token.RequestTime.Add(time.Duration(c.token.ExpiresIn)*time.Second-10*time.Second).After(time.Now()) {
		return *(c.token), nil
	}
	token, err = GetOpenidPasswordToken(c.authEndpoint, c.authClientId, c.authClientSecret)
	if err != nil {
		return token, err
	}
	c.token = &token
	return token, err
}

func (c *Client) GetRealmRoleMappings(userId string) (mappings []RoleMappings, err error) {
	token, err := c.GetToken()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, c.authEndpoint+"/admin/realms/master/users/"+userId+"/role-mappings/realm", nil)
	req.Header.Set("Authorization", token.JwtToken())
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		temp, _ := io.ReadAll(resp.Body) //read error response end ensure that resp.Body is read to EOF
		return nil, fmt.Errorf("unexpected statuscode %v: %v", resp.StatusCode, string(temp))
	}
	err = json.NewDecoder(resp.Body).Decode(&mappings)
	if err != nil {
		_, _ = io.ReadAll(resp.Body) //ensure resp.Body is read to EOF
		return nil, err
	}
	return mappings, nil
}

func (c *Client) GetUsers() (mappings []User, err error) {
	token, err := c.GetToken()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, c.authEndpoint+"/admin/realms/master/users", nil)
	req.Header.Set("Authorization", token.JwtToken())
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		temp, _ := io.ReadAll(resp.Body) //read error response end ensure that resp.Body is read to EOF
		return nil, fmt.Errorf("unexpected statuscode %v: %v", resp.StatusCode, string(temp))
	}
	err = json.NewDecoder(resp.Body).Decode(&mappings)
	if err != nil {
		_, _ = io.ReadAll(resp.Body) //ensure resp.Body is read to EOF
		return nil, err
	}
	return mappings, nil
}

func GetOpenidPasswordToken(authEndpoint string, authClientId string, authClientSecret string) (token OpenidToken, err error) {
	requesttime := time.Now()
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	values := url.Values{
		"client_id":     {authClientId},
		"client_secret": {authClientSecret},
		"grant_type":    {"client_credentials"},
	}

	resp, err := client.PostForm(authEndpoint+"/realms/master/protocol/openid-connect/token", values)

	if err != nil {
		return token, err
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		err = errors.New(resp.Status + ": " + string(b))
		return
	}
	err = json.NewDecoder(resp.Body).Decode(&token)
	token.RequestTime = requesttime
	return
}
