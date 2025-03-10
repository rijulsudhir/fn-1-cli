/*
 * Copyright (c) 2019, 2020 Oracle and/or its affiliates. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package langs

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// JavaLangHelper provides a set of helper methods for the lifecycle of Java Maven projects
type JavaLangHelper struct {
	BaseHelper
	version          string
	latestFdkVersion string
	pomType          string
}

func (h *JavaLangHelper) Handles(lang string) bool {
	return defaultHandles(h, lang)
}
func (h *JavaLangHelper) Runtime() string {
	return h.LangStrings()[0]
}

func (h *JavaLangHelper) LangStrings() []string {
	return []string{"java", fmt.Sprintf("java%s", h.version)}
}

func (h *JavaLangHelper) Extensions() []string {
	return []string{".java"}
}

// BuildFromImage returns the Docker image used to compile the Maven function project
func (h *JavaLangHelper) BuildFromImage() (string, error) {

	fdkVersion, err := h.GetLatestFDKVersion()
	if err != nil {
		return "", err
	}

	if h.version == "8" {
		return fmt.Sprintf("fnproject/fn-java-fdk-build:%s", fdkVersion), nil
	} else if h.version == "11" {
		return fmt.Sprintf("fnproject/fn-java-fdk-build:jdk11-%s", fdkVersion), nil
	} else {
		return "", fmt.Errorf("unsupported java version %s", h.version)
	}
}

// RunFromImage returns the Docker image used to run the Java function.
func (h *JavaLangHelper) RunFromImage() (string, error) {
	fdkVersion, err := h.GetLatestFDKVersion()
	if err != nil {
		return "", err
	}
	if h.version == "8" {
		return fmt.Sprintf("fnproject/fn-java-fdk:%s", fdkVersion), nil
	} else if h.version == "11" {
		return fmt.Sprintf("fnproject/fn-java-fdk:jre11-%s", fdkVersion), nil
	} else {
		return "", fmt.Errorf("unsupported java version %s", h.version)
	}
}

// HasBoilerplate returns whether the Java runtime has boilerplate that can be generated.
func (h *JavaLangHelper) HasBoilerplate() bool { return true }

// CustomMemory - no custom memory value here.
func (h *JavaLangHelper) CustomMemory() uint64 { return 0 }

// GenerateBoilerplate will generate function boilerplate for a Java runtime. The default boilerplate is for a Maven
// project.
func (h *JavaLangHelper) GenerateBoilerplate(path string) error {
	pathToPomFile := filepath.Join(path, "pom.xml")
	if exists(pathToPomFile) {
		return ErrBoilerplateExists
	}

	apiVersion, err := h.GetLatestFDKVersion()
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(pathToPomFile, []byte(pomFileContent(apiVersion, h.version, h.pomType)), os.FileMode(0644)); err != nil {
		return err
	}

	mkDirAndWriteFile := func(dir, filename, content string) error {
		fullPath := filepath.Join(path, dir)
		if err = os.MkdirAll(fullPath, os.FileMode(0755)); err != nil {
			return err
		}

		fullFilePath := filepath.Join(fullPath, filename)
		return ioutil.WriteFile(fullFilePath, []byte(content), os.FileMode(0644))
	}

	err = mkDirAndWriteFile("src/main/java/com/example/fn", "HelloFunction.java", helloJavaSrcBoilerplate)
	if err != nil {
		return err
	}

	return mkDirAndWriteFile("src/test/java/com/example/fn", "HelloFunctionTest.java", helloJavaTestBoilerplate)
}

// Cmd returns the Java runtime Docker entrypoint that will be executed when the function is executed.
func (h *JavaLangHelper) Cmd() (string, error) {
	return "com.example.fn.HelloFunction::handleRequest", nil
}

// DockerfileCopyCmds returns the Docker COPY command to copy the compiled Java function jar and dependencies.
func (h *JavaLangHelper) DockerfileCopyCmds() []string {
	return []string{
		"COPY --from=build-stage /function/target/*.jar /function/app/",
	}
}

// DockerfileBuildCmds returns the build stage steps to compile the Maven function project.
func (h *JavaLangHelper) DockerfileBuildCmds() []string {
	return []string{
		fmt.Sprintf("ENV MAVEN_OPTS %s", mavenOpts()),
		"ADD pom.xml /function/pom.xml",
		"RUN [\"mvn\", \"package\", \"dependency:copy-dependencies\", \"-DincludeScope=runtime\", " +
			"\"-DskipTests=true\", \"-Dmdep.prependGroupId=true\", \"-DoutputDirectory=target\", \"--fail-never\"]",
		"ADD src /function/src",
		"RUN [\"mvn\", \"package\"]",
	}
}

// HasPreBuild returns whether the Java Maven runtime has a pre-build step.
func (h *JavaLangHelper) HasPreBuild() bool { return true }

// PreBuild ensures that the expected the function is based is a maven project.
func (h *JavaLangHelper) PreBuild() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if !exists(filepath.Join(wd, "pom.xml")) {
		return errors.New("Could not find pom.xml - are you sure this is a Maven project?")
	}

	return nil
}

func mavenOpts() string {
	var opts bytes.Buffer

	if parsedURL, err := url.Parse(os.Getenv("http_proxy")); err == nil {
		opts.WriteString(fmt.Sprintf("-Dhttp.proxyHost=%s ", parsedURL.Hostname()))
		opts.WriteString(fmt.Sprintf("-Dhttp.proxyPort=%s ", parsedURL.Port()))
	}

	if parsedURL, err := url.Parse(os.Getenv("https_proxy")); err == nil {
		opts.WriteString(fmt.Sprintf("-Dhttps.proxyHost=%s ", parsedURL.Hostname()))
		opts.WriteString(fmt.Sprintf("-Dhttps.proxyPort=%s ", parsedURL.Port()))
	}

	nonProxyHost := os.Getenv("no_proxy")
	opts.WriteString(fmt.Sprintf("-Dhttp.nonProxyHosts=%s ", strings.Replace(nonProxyHost, ",", "|", -1)))

	opts.WriteString("-Dmaven.repo.local=/usr/share/maven/ref/repository")

	return opts.String()
}

/*    TODO temporarily generate maven project boilerplate from hardcoded values.
Will eventually move to using a maven archetype.
*/
func pomFileContent(APIversion, javaVersion, pomType string) string {
	if pomType == "maven" {
		return fmt.Sprintf(mavenPomFile, APIversion, javaVersion, javaVersion)
	} else {
		return fmt.Sprintf(bintrayPomFile, APIversion, javaVersion, javaVersion)
	}
}

func (h *JavaLangHelper) GetLatestFDKVersion() (string, error) {

	if h.latestFdkVersion != "" {
		return h.latestFdkVersion, nil
	}

	const bintrayVersionURL = "https://api.bintray.com/search/packages/maven?repo=fnproject&g=com.fnproject.fn&a=fdk"
	const mavenVersionUrl = "https://repo1.maven.org/maven2/com/fnproject/fn/fdk/maven-metadata.xml"

	const versionEnv = "FN_JAVA_FDK_VERSION"
	fetchError := fmt.Errorf("Failed to fetch latest Java FDK javaVersion. Check your network settings or manually override the javaVersion by setting %s", versionEnv)
	version := os.Getenv(versionEnv)

	if version != "" {
		return version, nil
	}
	version, pType, err := getFDKLatestFromURL(mavenVersionUrl, bintrayVersionURL)
	if err != nil {
		return "", fetchError
	}

	h.latestFdkVersion = version
	h.pomType = pType
	return version, nil
}

func getFDKLatestFromURL(comURL string, bintrayURL string) (string, string, error) {
	var data []byte
	var err error
	err = fmt.Errorf("All URL failed to respond ")

	//First search for com.fnproject.fn from Maven Central to get the latest version
	data, err = getURLResponse(comURL, false)
	if err == nil {
		version, e1 := parseMavenResponse(data)
		if e1 == nil {
			return version, "maven", e1
		}
	}

	//Second time search for com.fnproject.fn from Bintray to get the latest version, if fetch from Maven fails
	data, err = getURLResponse(bintrayURL, true)
	if err == nil {
		version, e1 := parseBintrayResponse(data)
		if e1 == nil {
			return version, "bintray", e1
		}
	}

	//In all other case return error as latest FDK version is not identified
	return "", "", err
}

func getURLResponse(url string, inSecureSkipVerify bool) ([]byte, error) {
	defaultTransport := http.DefaultTransport.(*http.Transport)
	// nishalad95: bin tray TLS certs cause verification issues on OSX, skip TLS verification
	noVerifyTransport := &http.Transport{
		Proxy:                 defaultTransport.Proxy,
		DialContext:           defaultTransport.DialContext,
		MaxIdleConns:          defaultTransport.MaxIdleConns,
		IdleConnTimeout:       defaultTransport.IdleConnTimeout,
		ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
		TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: inSecureSkipVerify},
	}
	client := &http.Client{Transport: noVerifyTransport}
	resp, err := client.Get(url)
	defer resp.Body.Close()
	if err != nil || resp.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to fetch response from URL %s Error: %v Status: %d", url, err, resp.StatusCode)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func parseMavenResponse(data []byte) (string, error) {
	type ParsedResponse struct {
		XMLName    xml.Name `xml:"metadata"`
		Text       string   `xml:",chardata"`
		GroupId    string   `xml:"groupId"`
		ArtifactId string   `xml:"artifactId"`
		Versioning struct {
			Text     string `xml:",chardata"`
			Latest   string `xml:"latest"`
			Release  string `xml:"release"`
			Versions struct {
				Text    string   `xml:",chardata"`
				Version []string `xml:"version"`
			} `xml:"versions"`
			LastUpdated string `xml:"lastUpdated"`
		} `xml:"versioning"`
	}
	var response ParsedResponse
	err := xml.Unmarshal(data, &response)
	if err != nil {
		return "", err
	}

	if len(response.Versioning.Versions.Version) == 0 {
		return "", fmt.Errorf("Maven response is not valid")
	}
	version := response.Versioning.Latest
	return version, nil
}

func parseBintrayResponse(data []byte) (string, error) {
	type parsedResponse struct {
		Version string `json:"latest_version"`
	}
	parsedResp := make([]parsedResponse, 1)
	err := json.Unmarshal(data, &parsedResp)
	if err != nil {
		return "", err
	}
	version := parsedResp[0].Version

	return version, nil
}

func (h *JavaLangHelper) FixImagesOnInit() bool {
	return true
}

const (
	mavenPomFile = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    <properties>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
        <fdk.version>%s</fdk.version>
    </properties>
    <groupId>com.example.fn</groupId>
    <artifactId>hello</artifactId>
    <version>1.0.0</version>

    <dependencies>
        <dependency>
            <groupId>com.fnproject.fn</groupId>
            <artifactId>api</artifactId>
            <version>${fdk.version}</version>
        </dependency>
        <dependency>
            <groupId>com.fnproject.fn</groupId>
            <artifactId>testing-core</artifactId>
            <version>${fdk.version}</version>
            <scope>test</scope>
        </dependency>
        <dependency>
            <groupId>com.fnproject.fn</groupId>
            <artifactId>testing-junit4</artifactId>
            <version>${fdk.version}</version>
            <scope>test</scope>
        </dependency>
        <dependency>
            <groupId>junit</groupId>
            <artifactId>junit</artifactId>
            <version>4.12</version>
            <scope>test</scope>
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-compiler-plugin</artifactId>
                <version>3.3</version>
                <configuration>
                    <source>%s</source>
                    <target>%s</target>
                </configuration>
            </plugin>
            <plugin>
                 <groupId>org.apache.maven.plugins</groupId>
                 <artifactId>maven-surefire-plugin</artifactId>
                 <version>2.22.1</version>
                 <configuration>
                     <useSystemClassLoader>false</useSystemClassLoader>
                 </configuration>
            </plugin>
        </plugins>
    </build>
</project>
`

	bintrayPomFile = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    <properties>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
        <fdk.version>%s</fdk.version>
    </properties>
    <groupId>com.example.fn</groupId>
    <artifactId>hello</artifactId>
    <version>1.0.0</version>

	<repositories>
        <repository>
            <id>fn-release-repo</id>
            <url>https://dl.bintray.com/fnproject/fnproject</url>
            <releases>
                <enabled>true</enabled>
            </releases>
            <snapshots>
                <enabled>false</enabled>
            </snapshots>
        </repository>
    </repositories>

    <dependencies>
        <dependency>
            <groupId>com.fnproject.fn</groupId>
            <artifactId>api</artifactId>
            <version>${fdk.version}</version>
        </dependency>
        <dependency>
            <groupId>com.fnproject.fn</groupId>
            <artifactId>testing-core</artifactId>
            <version>${fdk.version}</version>
            <scope>test</scope>
        </dependency>
        <dependency>
            <groupId>com.fnproject.fn</groupId>
            <artifactId>testing-junit4</artifactId>
            <version>${fdk.version}</version>
            <scope>test</scope>
        </dependency>
        <dependency>
            <groupId>junit</groupId>
            <artifactId>junit</artifactId>
            <version>4.12</version>
            <scope>test</scope>
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-compiler-plugin</artifactId>
                <version>3.3</version>
                <configuration>
                    <source>%s</source>
                    <target>%s</target>
                </configuration>
            </plugin>
            <plugin>
                 <groupId>org.apache.maven.plugins</groupId>
                 <artifactId>maven-surefire-plugin</artifactId>
                 <version>2.22.1</version>
                 <configuration>
                     <useSystemClassLoader>false</useSystemClassLoader>
                 </configuration>
            </plugin>
        </plugins>
    </build>
</project>
`

	helloJavaSrcBoilerplate = `package com.example.fn;

public class HelloFunction {

    public String handleRequest(String input) {
        String name = (input == null || input.isEmpty()) ? "world"  : input;

        System.out.println("Inside Java Hello World function"); 
        return "Hello, " + name + "!";
    }

}`

	helloJavaTestBoilerplate = `package com.example.fn;

import com.fnproject.fn.testing.*;
import org.junit.*;

import static org.junit.Assert.*;

public class HelloFunctionTest {

    @Rule
    public final FnTestingRule testing = FnTestingRule.createDefault();

    @Test
    public void shouldReturnGreeting() {
        testing.givenEvent().enqueue();
        testing.thenRun(HelloFunction.class, "handleRequest");

        FnResult result = testing.getOnlyResult();
        assertEquals("Hello, world!", result.getBodyAsString());
    }

}`
)
