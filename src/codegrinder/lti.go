package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/sessions"
	"github.com/russross/meddler"
)

// LTIRequest is an LTI request object (generated by Canvas or other LMS).
type LTIRequest struct {
	PersonNameFull                   string  `form:"lis_person_name_full"`                     // Russ Ross
	PersonNameFamily                 string  `form:"lis_person_name_family"`                   // Ross
	PersonNameGiven                  string  `form:"lis_person_name_given"`                    // Russ
	PersonContactEmailPrimary        string  `form:"lis_person_contact_email_primary"`         // russ@dixie.edu
	UserID                           string  `form:"user_id"`                                  // <opaque>: unique per user
	Roles                            string  `form:"roles"`                                    // Instructor, Student; note: varies per course
	UserImage                        string  `form:"user_image"`                               // https:// ... user picture
	LTIMessageType                   string  `form:"lti_message_type"`                         // basic-lti-launch-request
	LTIVersion                       string  `form:"lti_version"`                              // LTI-1p0
	LaunchPresentationDocumentTarget string  `form:"launch_presentation_document_target"`      // iframe
	LaunchPresentationLocale         string  `form:"launch_presentation_locale"`               // en
	TCInstanceName                   string  `form:"tool_consumer_instance_name"`              // Dixie State University
	TCInstanceGUID                   string  `form:"tool_consumer_instance_guid"`              // <opaque>: unique per Canvas instance
	TCInstanceContactEmail           string  `form:"tool_consumer_instance_contact_email"`     // notifications@instructure.com
	TCInstanceVersion                string  `form:"tool_consumer_info_version"`               // cloud
	TCInfoProductFamilyCode          string  `form:"tool_consumer_info_product_family_code"`   // canvas
	CourseOfferingSourceDID          string  `form:"lis_course_offering_sourcedid"`            // CCRSCS-3520-42527.201440
	ContextTitle                     string  `form:"context_title"`                            // CS-3520-01 FA14
	ContextLabel                     string  `form:"context_label"`                            // CS-3520
	ContextID                        string  `form:"context_id"`                               // <opaque>: unique per course
	ResourceLinkTitle                string  `form:"resource_link_title"`                      // Code Grinder
	ResourceLinkID                   string  `form:"resource_link_id"`                         // <opaque>: unique per course+link, i.e., per-assignment
	PersonSourcedID                  string  `form:"lis_result_sourcedid"`                     // <opaque>: unique per course+link+user, for grade callback
	OutcomeServiceURL                string  `form:"lis_outcome_service_url"`                  // https://... to post grade
	ExtIMSBasicOutcomeURL            string  `form:"ext_ims_lis_basic_outcome_url"`            // https://... to post grade with extensions
	ExtOutcomeDataValuesAccepted     string  `form:"ext_outcome_data_values_accepted"`         // url,text what can be passed back with grade
	LaunchPresentationReturnURL      string  `form:"launch_presentation_return_url"`           // https://... when finished
	CanvasUserLoginID                string  `form:"custom_canvas_user_login_id"`              // rross5
	CanvasAssignmentPointsPossible   float64 `form:"custom_canvas_assignment_points_possible"` // 10
	CanvasEnrollmentState            string  `form:"custom_canvas_enrollment_state"`           // active
	CanvasCourseID                   int     `form:"custom_canvas_course_id"`                  // 279080
	CanvasUserID                     int     `form:"custom_canvas_user_id"`                    // 353051
	CanvasAssignmentTitle            string  `form:"custom_canvas_assignment_title"`           // YouFace Template
	CanvasAssignmentID               int     `form:"custom_canvas_assignment_id"`              // 1566693
	CanvasAPIDomain                  string  `form:"custom_canvas_api_domain"`                 // dixie.instructure.com
	OAuthVersion                     string  `form:"oauth_version"`                            // 1.0
	OAuthSignature                   string  `form:"oauth_signature"`                          // <opaque> base64
	OAuthSignatureMethod             string  `form:"oauth_signature_method"`                   // HMAC-SHA1
	OAuthTimestamp                   int     `form:"oauth_timestamp"`                          // 1400000132 (unix seconds)
	OAuthConsumerKey                 string  `form:"oauth_consumer_key"`                       // cs3520 (what the instructor entered at setup time)
	OAuthNonce                       string  `form:"oauth_nonce"`                              // <opaque>: must only be accepted once
	OAuthCallback                    string  `form:"oauth_callback"`                           // about:blank
}

// GradeResponse is the XML format to post a grade back to the LMS.
type GradeResponse struct {
	XMLName   xml.Name `xml:"imsx_POXEnvelopeRequest"`
	Namespace string   `xml:"xmlns,attr"`
	Version   string   `xml:"imsx_POXHeader>imsx_POXRequestHeaderInfo>imsx_version"`
	Message   string   `xml:"imsx_POXHeader>imsx_POXRequestHeaderInfo>imsx_messageIdentifier"`
	SourcedID string   `xml:"imsx_POXBody>replaceResultRequest>resultRecord>sourcedGUID>sourcedId"`
	Language  string   `xml:"imsx_POXBody>replaceResultRequest>resultRecord>result>resultScore>language"`
	Score     string   `xml:"imsx_POXBody>replaceResultRequest>resultRecord>result>resultScore>textString"`
	URL       string   `xml:"imsx_POXBody>replaceResultRequest>resultRecord>result>resultData>url,omitempty"`
	Text      string   `xml:"imsx_POXBody>replaceResultRequest>resultRecord>result>resultData>text,omitempty"`
}

// LTIConfig is the XML format to configure the LMS to use this tool.
type LTIConfig struct {
	XMLName         xml.Name            `xml:"cartridge_basiclti_link"`
	Namespace       string              `xml:"xmlns,attr"`
	NamespaceBLTI   string              `xml:"xmlns:blti,attr"`
	NamespaceLTICM  string              `xml:"xmlns:lticm,attr"`
	NamespaceLTICP  string              `xml:"xmlns:lticp,attr"`
	NamespaceXSI    string              `xml:"xmlns:xsi,attr"`
	SchemaLocation  string              `xml:"xsi:schemaLocation,attr"`
	Title           string              `xml:"blti:title"`
	Description     string              `xml:"blti:description"`
	Icon            string              `xml:"blti:icon"`
	Extensions      LTIConfigExtensions `xml:"blti:extensions"`
	CartridgeBundle LTICartridge        `xml:"cartridge_bundle"`
	CartridgeIcon   LTICartridge        `xml:"cartridge_icon"`
}

// LTIConfigExtensions is the XML format for Canvas extensions to LTI configuration.
type LTIConfigExtensions struct {
	Platform   string `xml:"platform,attr"`
	Extensions []LTIConfigExtension
	Options    []LTIConfigOptions
}

// LTIConfigOptions is part of the XML format for Canvas extensions to LTI configuration.
type LTIConfigOptions struct {
	XMLName xml.Name `xml:"lticm:options"`
	Name    string   `xml:"name,attr"`
	Options []LTIConfigExtension
}

// LTIConfigExtension is part of the XML format for Canvas extensions to LTI configuration.
type LTIConfigExtension struct {
	XMLName xml.Name `xml:"lticm:property"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:",chardata"`
}

// LTICartridge is part of the XML format for Canvas extensions to LTI configuration.
type LTICartridge struct {
	IdentifierRef string `xml:"identifierref,attr"`
}

// GetConfigXML handles /lti/config.xml requests, returning an XML file to configure the LMS to use this tool.
func GetConfigXML(w http.ResponseWriter) {
	c := &LTIConfig{
		Namespace:      "http://www.imsglobal.org/xsd/imslticc_v1p0",
		NamespaceBLTI:  "http://www.imsglobal.org/xsd/imsbasiclti_v1p0",
		NamespaceLTICM: "http://www.imsglobal.org/xsd/imslticm_v1p0",
		NamespaceLTICP: "http://www.imsglobal.org/xsd/imslticp_v1p0",
		NamespaceXSI:   "http://www.w3.org/2001/XMLSchema-instance",
		SchemaLocation: "http://www.imsglobal.org/xsd/imslticc_v1p0 http://www.imsglobal.org/xsd/lti/ltiv1p0/imslticc_v1p0.xsd" +
			" http://www.imsglobal.org/xsd/imsbasiclti_v1p0 http://www.imsglobal.org/xsd/lti/ltiv1p0/imsbasiclti_v1p0.xsd" +
			" http://www.imsglobal.org/xsd/imslticm_v1p0 http://www.imsglobal.org/xsd/lti/ltiv1p0/imslticm_v1p0.xsd" +
			" http://www.imsglobal.org/xsd/imslticp_v1p0 http://www.imsglobal.org/xsd/lti/ltiv1p0/imslticp_v1p0.xsd",
		Title:       Config.ToolName,
		Description: Config.ToolDescription,
		Extensions: LTIConfigExtensions{
			Platform: "canvas.instructure.com",
			Extensions: []LTIConfigExtension{
				LTIConfigExtension{Name: "tool_id", Value: Config.ToolID},
				LTIConfigExtension{Name: "privacy_level", Value: "public"},
				LTIConfigExtension{Name: "domain", Value: Config.PublicURL[len("https://"):]},
			},
			Options: []LTIConfigOptions{
				LTIConfigOptions{
					Name: "resource_selection",
					Options: []LTIConfigExtension{
						LTIConfigExtension{Name: "url", Value: Config.PublicURL + "/lti/problems"},
						LTIConfigExtension{Name: "text", Value: Config.ToolName},
						LTIConfigExtension{Name: "selection_width", Value: "320"},
						LTIConfigExtension{Name: "selection_height", Value: "640"},
						LTIConfigExtension{Name: "enabled", Value: "true"},
					},
				},
			},
		},
		CartridgeBundle: LTICartridge{IdentifierRef: "BLTI001_Bundle"},
		CartridgeIcon:   LTICartridge{IdentifierRef: "BLTI001_Icon"},
	}
	raw, err := xml.MarshalIndent(c, "", "  ")
	if err != nil {
		loge.Printf("error rendering XML config data: %v", err)
		http.Error(w, "Error rendering XML", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	if _, err = fmt.Fprintf(w, "%s%s\n", xml.Header, raw); err != nil {
		loge.Printf("error writing XML: %v", err)
		http.Error(w, "Error writing XML", http.StatusInternalServerError)
		return
	}
}

func signXMLRequest(consumerKey, method, targetUrl, content, secret string) string {
	sum := sha1.Sum([]byte(content))
	bodyHash := base64.StdEncoding.EncodeToString(sum[:])

	// gather parts as form value for the signature
	v := url.Values{}
	v.Set("oauth_body_hash", bodyHash)
	v.Set("oauth_token", "")
	v.Set("oauth_consumer_key", consumerKey)
	v.Set("oauth_signature_method", "HMAC-SHA1")
	v.Set("oauth_timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	v.Set("oauth_version", "1.0")
	v.Set("oauth_nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// compute the signature and add it to the mix
	sig := computeOAuthSignature(method, targetUrl, v, secret)
	v.Set("oauth_signature", sig)

	// form the Authorization header
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf(`OAuth realm="%s"`, escape(Config.PublicURL)))
	for key, val := range v {
		buf.WriteString(fmt.Sprintf(`,%s="%s"`, key, escape(val[0])))
	}
	return buf.String()
}

func getMyURL(r *http.Request, withPath bool) *url.URL {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "https"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	u := &url.URL{
		Scheme: scheme,
		Host:   host,
	}
	if withPath {
		u.Path = r.URL.Path
	}
	return u
}

func checkOAuthSignature(w http.ResponseWriter, r *http.Request) {
	// make sure this is a signed request
	r.ParseForm()
	expected := r.Form.Get("oauth_signature")
	if expected == "" {
		loge.Printf("Missing oauth_signature form field")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// compute the signature
	sig := computeOAuthSignature(r.Method, getMyURL(r, true).String(), r.Form, Config.OAuthSharedSecret)

	// verify it
	if sig != expected {
		loge.Printf("Signature mismatch: got %s but expected %s", sig, expected)
		w.WriteHeader(http.StatusUnauthorized)
	}

	//logi.Printf("Signature %s checks out", sig)
}

func computeOAuthSignature(method, urlString string, parameters url.Values, secret string) string {
	// method must be upper case
	method = strings.ToUpper(method)

	// make sure scheme and host are lower case
	u, err := url.Parse(urlString)
	if err != nil {
		loge.Printf("Error parsing URI: %v", err)
		return ""
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Opaque = ""
	u.User = nil
	u.Host = strings.ToLower(u.Host)
	u.RawQuery = ""
	u.Fragment = ""
	reqURL := u.String()

	// get a sorted list of parameter keys (minus oauth_signature)
	oldsig := parameters.Get("oauth_signature")
	parameters.Del("oauth_signature")
	params := encode(parameters)
	if oldsig != "" {
		parameters.Set("oauth_signature", oldsig)
	}

	// get the full string
	s := escape(method) + "&" + escape(reqURL) + "&" + escape(params)

	// perform the signature
	// key is a combination of consumer secret and token secret, but we don't have token secrets
	mac := hmac.New(sha1.New, []byte(escape(secret)+"&"))
	mac.Write([]byte(s))
	sum := mac.Sum(nil)

	return base64.StdEncoding.EncodeToString(sum)
}

func escape(s string) string {
	var buf bytes.Buffer
	for _, b := range []byte(s) {
		if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-' || b == '.' || b == '_' || b == '~' {
			buf.WriteByte(b)
		} else {
			fmt.Fprintf(&buf, "%%%02X", b)
		}
	}
	return buf.String()
}

// this is url.URL.Encode from the standard library, but using escape instead of url.QueryEscape
func encode(v url.Values) string {
	if v == nil {
		return ""
	}
	var buf bytes.Buffer
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vs := v[k]
		prefix := escape(k) + "="
		for _, v := range vs {
			if buf.Len() > 0 {
				buf.WriteByte('&')
			}
			buf.WriteString(prefix)
			buf.WriteString(escape(v))
		}
	}
	return buf.String()
}

// LtiProblem handles /lti/problem/:unique requests.
// It creates the user/course/assignment if necessary, creates a session,
// and redirects the user to the main UI URL.
func LtiProblem(w http.ResponseWriter, r *http.Request, db *sql.Tx, form LTIRequest, params martini.Params, session sessions.Session) {
	unique := params["unique"]
	if unique == "" {
		loge.Print(HTTPErrorf(w, http.StatusBadRequest, "Malformed URL: missing unique ID for problem"))
		return
	}
	if unique != url.QueryEscape(unique) {
		loge.Print(HTTPErrorf(w, http.StatusBadRequest, "unique ID must be URL friendly: %s is escaped as %s", unique, url.QueryEscape(unique)))
		return
	}

	now := time.Now()

	// load the problem
	problem := new(Problem)
	if err := meddler.QueryRow(db, problem, `SELECT * FROM problems WHERE unique_id = $1`, unique); err != nil {
		if err == sql.ErrNoRows {
			// no such problem
			loge.Print(HTTPErrorf(w, http.StatusNotFound, "no problem found with ID %s", unique))
			return
		}
		loge.Print(HTTPErrorf(w, http.StatusInternalServerError, "db error loading problem %s: %v", unique, err))
		return
	}

	// load the course
	course, err := getUpdateCourse(db, &form, now)
	if err != nil {
		http.Error(w, "DB error getting course", http.StatusInternalServerError)
		return
	}

	// load the user
	user, err := getUpdateUser(db, &form, now)
	if err != nil {
		http.Error(w, "DB error getting user", http.StatusInternalServerError)
		return
	}

	// load the assignment
	asst, err := getUpdateAssignment(db, &form, now, course, problem, user)
	if err != nil {
		http.Error(w, "DB error getting assignment", http.StatusInternalServerError)
		return
	}

	// sign the user in
	session.Set("user_id", user.ID)

	// redirect to the console
	http.Redirect(w, r, fmt.Sprintf("/#/assignment/%d", asst.ID), http.StatusSeeOther)
}

// LtiProblems handles /lti/problems requests.
// It creates the user/course if necessary, creates a session,
// and redirects the user to the problem picker UI URL.
func LtiProblems(w http.ResponseWriter, r *http.Request, db *sql.Tx, form LTIRequest, render render.Render, session sessions.Session) {
	now := time.Now()

	// load the coarse
	if _, err := getUpdateCourse(db, &form, now); err != nil {
		http.Error(w, "DB error getting course", http.StatusInternalServerError)
		return
	}

	// load the user
	user, err := getUpdateUser(db, &form, now)
	if err != nil {
		http.Error(w, "DB error getting user", http.StatusInternalServerError)
		return
	}

	// sign the user in
	session.Set("user_id", user.ID)

	u := &url.URL{
		Path: "/",
		Fragment: fmt.Sprintf("/problems/%s/%s",
			url.QueryEscape(form.OAuthConsumerKey),
			url.QueryEscape(form.LaunchPresentationReturnURL)),
	}

	logi.Printf("problem picker redirecting to %s", u.String())
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// get/create/update this user
func getUpdateUser(db *sql.Tx, form *LTIRequest, now time.Time) (*User, error) {
	user := new(User)
	if err := meddler.QueryRow(db, user, `SELECT * FROM users WHERE lti_id = $1`, form.UserID); err != nil {
		if err != sql.ErrNoRows {
			loge.Printf("db error loading user %s (%s): %v", form.UserID, form.PersonContactEmailPrimary, err)
			return nil, err
		}
		logi.Printf("creating new user (%s)", form.PersonContactEmailPrimary)
		user.ID = 0
		user.CreatedAt = now
		user.UpdatedAt = now
	}
	oldUser := new(User)
	*oldUser = *user
	user.Name = form.PersonNameFull
	user.Email = form.PersonContactEmailPrimary
	user.LtiID = form.UserID
	user.ImageURL = form.UserImage
	user.CanvasLogin = form.CanvasUserLoginID
	user.CanvasID = form.CanvasUserID
	if user.ID > 0 && *user != *oldUser {
		// if something changed, note the update time
		logi.Printf("user %d (%s) updated", user.ID, user.Email)
		user.UpdatedAt = now
	}

	// always save to note the last signed in time
	user.LastSignedInAt = now
	if err := meddler.Save(db, "users", user); err != nil {
		loge.Printf("db error updating user %s (%s): %v", user.LtiID, user.Email, err)
		return nil, err
	}

	return user, nil
}

// get/create/update this course
func getUpdateCourse(db *sql.Tx, form *LTIRequest, now time.Time) (*Course, error) {
	course := new(Course)
	if err := meddler.QueryRow(db, course, `SELECT * FROM courses WHERE lti_id = $1`, form.ContextID); err != nil {
		if err != sql.ErrNoRows {
			loge.Printf("db error loading course %s (%s): %v", form.ContextID, form.ContextTitle, err)
			return nil, err
		}
		logi.Printf("creating new course %s (%s)", form.ContextID, form.ContextTitle)
		course.ID = 0
		course.CreatedAt = now
		course.UpdatedAt = now
	}
	oldCourse := new(Course)
	*oldCourse = *course
	course.Name = form.ContextTitle
	course.Label = form.ContextLabel
	course.LtiID = form.ContextID
	course.CanvasID = form.CanvasCourseID
	if course.ID < 1 || *course != *oldCourse {
		// if something changed, note the update time and save
		if course.ID > 0 {
			logi.Printf("course %d (%s) updated", course.ID, course.Name)
		}
		course.UpdatedAt = now
		if err := meddler.Save(db, "courses", course); err != nil {
			loge.Printf("db error saving course %s (%s): %v", course.LtiID, course.Name, err)
			return nil, err
		}
	}

	return course, nil
}

// get/create/update this assignment
func getUpdateAssignment(db *sql.Tx, form *LTIRequest, now time.Time, course *Course, problem *Problem, user *User) (*Assignment, error) {
	asst := new(Assignment)
	err := meddler.QueryRow(db, asst, `SELECT * FROM assignments WHERE course_id = $1 AND problem_id = $2 AND user_id = $3`,
		course.ID, problem.ID, user.ID)
	if err != nil {
		if err != sql.ErrNoRows {
			loge.Printf("db error loading assignment for course %d, problem %d, user %d: %v", course.ID, problem.ID, user.ID, err)
			return nil, err
		}

		logi.Printf("creating new assignment for course %d (%s), problem %d (%s), user %d: %s (%s)",
			course.ID, course.Name, problem.ID, problem.Name, user.ID, user.Name, user.Email)
		asst.ID = 0
		asst.CreatedAt = now
		asst.UpdatedAt = now
	}
	oldAsst := new(Assignment)
	*oldAsst = *asst
	asst.CourseID = course.ID
	asst.ProblemID = problem.ID
	asst.UserID = user.ID
	asst.Roles = form.Roles
	asst.Points = form.CanvasAssignmentPointsPossible
	if form.PersonSourcedID != "" {
		asst.GradeID = form.PersonSourcedID
	}
	asst.LtiID = form.ResourceLinkID
	asst.CanvasTitle = form.CanvasAssignmentTitle
	asst.CanvasID = form.CanvasAssignmentID
	asst.CanvasAPIDomain = form.CanvasAPIDomain
	asst.OutcomeURL = form.OutcomeServiceURL
	asst.OutcomeExtURL = form.ExtIMSBasicOutcomeURL
	asst.OutcomeExtAccepted = form.ExtOutcomeDataValuesAccepted
	asst.FinishedURL = form.LaunchPresentationReturnURL
	asst.ConsumerKey = form.OAuthConsumerKey
	if asst.ID < 1 || *asst != *oldAsst {
		// if something changed, note the update time and save
		if asst.ID > 0 {
			logi.Printf("assignment %d (course %d (%s), problem %d (%s), user %d (%s) updated",
				asst.ID, course.ID, course.Name, problem.ID, problem.Name, user.ID, user.Email)
		}
		asst.UpdatedAt = now
		if err := meddler.Save(db, "assignments", asst); err != nil {
			loge.Printf("db error saving assignment for course %d, problem %d, user %d: %v", course.ID, problem.ID, user.ID, err)
			loge.Printf("LtiID (resource_link_id) = %v, GradeID = %v", asst.LtiID, asst.GradeID)

			// dump the request to the logs for debugging purposes
			if raw, err := json.MarshalIndent(form, ">>>>", "    "); err == nil {
				logi.Printf("LTI Request dump:")
				for _, line := range strings.Split(string(raw), "\n") {
					logi.Print(line)
				}
			}

			return nil, err
		}
	}

	return asst, nil
}

func saveGrade(db *sql.Tx, commit *Commit) error {
	if commit.ReportCard == nil {
		return nil
	}

	// get the assignment
	asst := new(Assignment)
	if err := meddler.QueryRow(db, asst, `SELECT * FROM assignments WHERE id = $1`, commit.AssignmentID); err != nil {
		loge.Printf("db error getting assignment %d associated with commit %d: %v", commit.AssignmentID, commit.ID, err)
		return err
	}
	if asst.GradeID == "" {
		logi.Printf("cannot post grade for assignment %d user %d because no grade ID is present", asst.ID, asst.UserID)
		return nil
	}
	if asst.OutcomeURL == "" {
		logi.Printf("cannot post grade for assignment %d user %d because no outcome URL is present", asst.ID, asst.UserID)
		return nil
	}

	// get the user
	user := new(User)
	if err := meddler.Load(db, "users", user, int64(asst.UserID)); err != nil {
		loge.Printf("db error getting user %d: %v", asst.UserID, err)
		return err
	}

	// get grading fields from each step in this problem
	var steps []*ProblemStep
	err := meddler.QueryAll(db, &steps, `SELECT id, position, score_weight FROM problem_steps WHERE problem_id = $1 ORDER BY position`, asst.ProblemID)
	if err != nil {
		loge.Printf("db error getting problem step weights for problem %d: %v", asst.ProblemID, err)
		return err
	}

	// assign a grade: all previous steps get full credit, this one gets partial credit, future steps get none
	score, possible := 0.0, 0.0
	foundCurrent := false
	for _, step := range steps {
		possible += step.ScoreWeight
		if step.ID == commit.ProblemStepID {
			if commit.ReportCard.Passed {
				// award full credit for this step
				score += step.ScoreWeight
			} else if len(commit.ReportCard.Results) == 0 {
				// no results? that's a fail...
			} else {
				// compute partial credit for this step
				passed := 0
				for _, elt := range commit.ReportCard.Results {
					if elt.Outcome == "passed" {
						passed++
					}
				}
				partial := float64(passed) * step.ScoreWeight / float64(len(commit.ReportCard.Results))
				score += partial
				//logi.Printf("passed %d/%d on this step", passed, len(commit.ReportCard.Results))
			}
			foundCurrent = true
		} else if !foundCurrent {
			// award full credit for completed steps
			score += step.ScoreWeight
		} else {
			// no credit for future steps
		}
	}

	// compute the weighted grade
	grade := 0.0
	if possible > 0.0 {
		grade = score / possible
	}

	// report back using lti
	outcomeURL := asst.OutcomeURL
	gradeURL := ""
	gradeText := ""
	/*
		if strings.Contains(asst.OutcomeExtAccepted, "url") {
			outcomeURL = asst.OutcomeExtURL
			gradeURL = "https://www.google.com/"
		}
	*/
	report := &GradeResponse{
		Namespace: "http://www.imsglobal.org/services/ltiv1p1/xsd/imsoms_v1p0",
		Version:   "V1.0",
		Message:   "Grade from Code Grinder",
		SourcedID: asst.GradeID,
		URL:       gradeURL,
		Text:      gradeText,
		Language:  "en",
		Score:     fmt.Sprintf("%0.4f", grade),
	}

	raw, err := xml.MarshalIndent(report, "", "  ")
	if err != nil {
		loge.Printf("error rendering XML grade response: %v", err)
		return err
	}
	result := fmt.Sprintf("%s%s", xml.Header, raw)

	// sign the request
	auth := signXMLRequest(asst.ConsumerKey, "POST", outcomeURL, result, Config.OAuthSharedSecret)

	// POST the grade
	req, err := http.NewRequest("POST", outcomeURL, strings.NewReader(result))
	if err != nil {
		loge.Printf("error preparing grade request: %v", err)
		return err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/xml")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		loge.Printf("error sending grade request: %v", err)
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		logi.Printf("grade of %0.4f posted for %s (%s)", grade, user.Name, user.Email)
	} else {
		return loggedErrorf("result status %d (%s) when posting grade for user %d", resp.StatusCode, resp.Status, asst.UserID)
	}

	return nil
}
