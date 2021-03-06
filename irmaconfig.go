package irma

import (
	"encoding/base64"
	"encoding/xml"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"crypto/sha256"

	"fmt"

	"strings"

	"sort"

	"bytes"

	"encoding/hex"

	"crypto/ecdsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"math/big"

	"github.com/go-errors/errors"
	"github.com/mhe/gabi"
	"github.com/privacybydesign/irmago/internal/fs"
)

// Configuration keeps track of scheme managers, issuers, credential types and public keys,
// dezerializing them from an irma_configuration folder, and downloads and saves new ones on demand.
type Configuration struct {
	SchemeManagers  map[SchemeManagerIdentifier]*SchemeManager
	Issuers         map[IssuerIdentifier]*Issuer
	CredentialTypes map[CredentialTypeIdentifier]*CredentialType

	// Path to the irma_configuration folder that this instance represents
	Path string

	// DisabledSchemeManagers keeps track of scheme managers that did not parse  succesfully
	// (i.e., invalid signature, parsing error), and the problem that occurred when parsing them
	DisabledSchemeManagers map[SchemeManagerIdentifier]*SchemeManagerError

	publicKeys    map[IssuerIdentifier]map[int]*gabi.PublicKey
	reverseHashes map[string]CredentialTypeIdentifier
	initialized   bool
	assets        string
}

// ConfigurationFileHash encodes the SHA256 hash of an authenticated
// file under a scheme manager within the configuration folder.
type ConfigurationFileHash []byte

// SchemeManagerIndex is a (signed) list of files under a scheme manager
// along with their SHA266 hash
type SchemeManagerIndex map[string]ConfigurationFileHash

type SchemeManagerStatus string

type SchemeManagerError struct {
	Manager SchemeManagerIdentifier
	Status  SchemeManagerStatus
	Err     error
}

const (
	SchemeManagerStatusValid               = SchemeManagerStatus("Valid")
	SchemeManagerStatusUnprocessed         = SchemeManagerStatus("Unprocessed")
	SchemeManagerStatusInvalidIndex        = SchemeManagerStatus("InvalidIndex")
	SchemeManagerStatusInvalidSignature    = SchemeManagerStatus("InvalidSignature")
	SchemeManagerStatusParsingError        = SchemeManagerStatus("ParsingError")
	SchemeManagerStatusContentParsingError = SchemeManagerStatus("ContentParsingError")
)

func (sme SchemeManagerError) Error() string {
	return fmt.Sprintf("Error parsing scheme manager %s: %s", sme.Manager.Name(), sme.Err.Error())
}

// NewConfiguration returns a new configuration. After this
// ParseFolder() should be called to parse the specified path.
func NewConfiguration(path string, assets string) (conf *Configuration, err error) {
	conf = &Configuration{
		Path:   path,
		assets: assets,
	}

	if err = fs.EnsureDirectoryExists(conf.Path); err != nil {
		return nil, err
	}
	isUpToDate, err := conf.isUpToDate()
	if err != nil {
		return nil, err
	}
	if conf.assets != "" && !isUpToDate {
		if err = conf.CopyFromAssets(false); err != nil {
			return nil, err
		}
	}

	// Init all maps
	conf.clear()

	return
}

func (conf *Configuration) clear() {
	conf.SchemeManagers = make(map[SchemeManagerIdentifier]*SchemeManager)
	conf.Issuers = make(map[IssuerIdentifier]*Issuer)
	conf.CredentialTypes = make(map[CredentialTypeIdentifier]*CredentialType)
	conf.DisabledSchemeManagers = make(map[SchemeManagerIdentifier]*SchemeManagerError)
	conf.publicKeys = make(map[IssuerIdentifier]map[int]*gabi.PublicKey)
	conf.reverseHashes = make(map[string]CredentialTypeIdentifier)
}

// ParseFolder populates the current Configuration by parsing the storage path,
// listing the containing scheme managers, issuers and credential types.
func (conf *Configuration) ParseFolder() (err error) {
	// Init all maps
	conf.clear()

	var mgrerr *SchemeManagerError
	err = iterateSubfolders(conf.Path, func(dir string) error {
		manager := NewSchemeManager(filepath.Base(dir))
		err := conf.ParseSchemeManagerFolder(dir, manager)
		if err == nil {
			return nil // OK, do next scheme manager folder
		}
		// If there is an error, and it is of type SchemeManagerError, return nil
		// so as to continue parsing other managers.
		var ok bool
		if mgrerr, ok = err.(*SchemeManagerError); ok {
			conf.DisabledSchemeManagers[manager.Identifier()] = mgrerr
			return nil
		}
		return err // Not a SchemeManagerError? return it & halt parsing now
	})
	if err != nil {
		return
	}
	conf.initialized = true
	if mgrerr != nil {
		return mgrerr
	}
	return
}

func (conf *Configuration) ParseOrRestoreFolder() error {
	err := conf.ParseFolder()
	var parse bool
	for id := range conf.DisabledSchemeManagers {
		parse = conf.CopyManagerFromAssets(id)
	}
	if parse {
		return conf.ParseFolder()
	}
	return err
}

// ParseSchemeManagerFolder parses the entire tree of the specified scheme manager
// If err != nil then a problem occured
func (conf *Configuration) ParseSchemeManagerFolder(dir string, manager *SchemeManager) (err error) {
	// From this point, keep it in our map even if it has an error. The user must check either:
	// - manager.Status == SchemeManagerStatusValid, aka "VALID"
	// - or equivalently, manager.Valid == true
	// before using any scheme manager for anything, and handle accordingly
	conf.SchemeManagers[manager.Identifier()] = manager

	// Ensure we return a SchemeManagerError when any error occurs
	defer func() {
		if err != nil {
			err = &SchemeManagerError{
				Manager: manager.Identifier(),
				Err:     err,
				Status:  manager.Status,
			}
		}
	}()

	err = fs.AssertPathExists(dir + "/description.xml")
	if err != nil {
		return
	}

	if manager.index, err = conf.parseIndex(filepath.Base(dir), manager); err != nil {
		manager.Status = SchemeManagerStatusInvalidIndex
		return
	}

	err = conf.VerifySchemeManager(manager)
	if err != nil {
		manager.Status = SchemeManagerStatusInvalidSignature
		return
	}

	exists, err := conf.pathToDescription(manager, dir+"/description.xml", manager)
	if !exists {
		manager.Status = SchemeManagerStatusParsingError
		return errors.New("Scheme manager description not found")
	}
	if err != nil {
		manager.Status = SchemeManagerStatusParsingError
		return
	}

	if manager.XMLVersion < 7 {
		manager.Status = SchemeManagerStatusParsingError
		return errors.New("Unsupported scheme manager description")
	}

	err = conf.parseIssuerFolders(manager, dir)
	if err != nil {
		manager.Status = SchemeManagerStatusContentParsingError
		return
	}
	manager.Status = SchemeManagerStatusValid
	manager.Valid = true
	return
}

func relativePath(absolute string, relative string) string {
	return relative[len(absolute)+1:]
}

// PublicKey returns the specified public key, or nil if not present in the Configuration.
func (conf *Configuration) PublicKey(id IssuerIdentifier, counter int) (*gabi.PublicKey, error) {
	if _, contains := conf.publicKeys[id]; !contains {
		conf.publicKeys[id] = map[int]*gabi.PublicKey{}
		if err := conf.parseKeysFolder(conf.SchemeManagers[id.SchemeManagerIdentifier()], id); err != nil {
			return nil, err
		}
	}
	return conf.publicKeys[id][counter], nil
}

func (conf *Configuration) addReverseHash(credid CredentialTypeIdentifier) {
	hash := sha256.Sum256([]byte(credid.String()))
	conf.reverseHashes[base64.StdEncoding.EncodeToString(hash[:16])] = credid
}

func (conf *Configuration) hashToCredentialType(hash []byte) *CredentialType {
	if str, exists := conf.reverseHashes[base64.StdEncoding.EncodeToString(hash)]; exists {
		return conf.CredentialTypes[str]
	}
	return nil
}

// IsInitialized indicates whether this instance has successfully been initialized.
func (conf *Configuration) IsInitialized() bool {
	return conf.initialized
}

// Prune removes any invalid scheme managers and everything they own from this Configuration
func (conf *Configuration) Prune() {
	for _, manager := range conf.SchemeManagers {
		if !manager.Valid {
			_ = conf.RemoveSchemeManager(manager.Identifier(), false) // does not return errors
		}
	}
}

func (conf *Configuration) parseIssuerFolders(manager *SchemeManager, path string) error {
	return iterateSubfolders(path, func(dir string) error {
		issuer := &Issuer{}
		exists, err := conf.pathToDescription(manager, dir+"/description.xml", issuer)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		if issuer.XMLVersion < 4 {
			return errors.New("Unsupported issuer description")
		}
		conf.Issuers[issuer.Identifier()] = issuer
		issuer.Valid = conf.SchemeManagers[issuer.SchemeManagerIdentifier()].Valid
		return conf.parseCredentialsFolder(manager, dir+"/Issues/")
	})
}

// parse $schememanager/$issuer/PublicKeys/$i.xml for $i = 1, ...
func (conf *Configuration) parseKeysFolder(manager *SchemeManager, issuerid IssuerIdentifier) error {
	path := fmt.Sprintf("%s/%s/%s/PublicKeys/*.xml", conf.Path, issuerid.SchemeManagerIdentifier().Name(), issuerid.Name())
	files, err := filepath.Glob(path)
	if err != nil {
		return err
	}

	for _, file := range files {
		filename := filepath.Base(file)
		count := filename[:len(filename)-4]
		i, err := strconv.Atoi(count)
		if err != nil {
			continue
		}
		bts, found, err := conf.ReadAuthenticatedFile(manager, relativePath(conf.Path, file))
		if err != nil || !found {
			return err
		}
		pk, err := gabi.NewPublicKeyFromBytes(bts)
		if err != nil {
			return err
		}
		pk.Issuer = issuerid.String()
		conf.publicKeys[issuerid][i] = pk
	}

	return nil
}

// parse $schememanager/$issuer/Issues/*/description.xml
func (conf *Configuration) parseCredentialsFolder(manager *SchemeManager, path string) error {
	return iterateSubfolders(path, func(dir string) error {
		cred := &CredentialType{}
		exists, err := conf.pathToDescription(manager, dir+"/description.xml", cred)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		if cred.XMLVersion < 4 {
			return errors.New("Unsupported credential type description")
		}
		cred.Valid = conf.SchemeManagers[cred.SchemeManagerIdentifier()].Valid
		credid := cred.Identifier()
		conf.CredentialTypes[credid] = cred
		conf.addReverseHash(credid)
		return nil
	})
}

// iterateSubfolders iterates over the subfolders of the specified path,
// calling the specified handler each time. If anything goes wrong, or
// if the caller returns a non-nil error, an error is immediately returned.
func iterateSubfolders(path string, handler func(string) error) error {
	dirs, err := filepath.Glob(path + "/*")
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		stat, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if !stat.IsDir() {
			continue
		}
		if strings.HasSuffix(dir, "/.git") {
			continue
		}
		err = handler(dir)
		if err != nil {
			return err
		}
	}

	return nil
}

func (conf *Configuration) pathToDescription(manager *SchemeManager, path string, description interface{}) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		return false, nil
	}

	bts, found, err := conf.ReadAuthenticatedFile(manager, relativePath(conf.Path, path))
	if !found {
		return false, nil
	}
	if err != nil {
		return true, err
	}

	err = xml.Unmarshal(bts, description)
	if err != nil {
		return true, err
	}

	return true, nil
}

// Contains checks if the configuration contains the specified credential type.
func (conf *Configuration) Contains(cred CredentialTypeIdentifier) bool {
	return conf.SchemeManagers[cred.IssuerIdentifier().SchemeManagerIdentifier()] != nil &&
		conf.Issuers[cred.IssuerIdentifier()] != nil &&
		conf.CredentialTypes[cred] != nil
}

func (conf *Configuration) readTimestamp(path string) (timestamp *time.Time, exists bool, err error) {
	filename := filepath.Join(path, "timestamp")
	exists, err = fs.PathExists(filename)
	if err != nil || !exists {
		return
	}
	bts, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}
	i, err := strconv.ParseInt(string(bts), 10, 64)
	if err != nil {
		return
	}
	t := time.Unix(i, 0)
	return &t, true, nil
}

func (conf *Configuration) isUpToDate() (bool, error) {
	if conf.assets == "" {
		return true, nil
	}
	var err error
	newTime, exists, err := conf.readTimestamp(conf.assets)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, errors.New("Timestamp in assets irma_configuration not found")
	}

	// conf.Path does not need to have a timestamp. If it does not, it is outdated
	oldTime, exists, err := conf.readTimestamp(conf.Path)
	return exists && !newTime.After(*oldTime), err
}

// CopyFromAssets recursively copies the directory tree from the assets folder
// into the directory of this Configuration.
func (conf *Configuration) CopyFromAssets(parse bool) error {
	if conf.assets == "" {
		return nil
	}
	if err := fs.CopyDirectory(conf.assets, conf.Path); err != nil {
		return err
	}
	if parse {
		return conf.ParseFolder()
	}
	return nil
}

func (conf *Configuration) CopyManagerFromAssets(managerID SchemeManagerIdentifier) bool {
	manager := conf.SchemeManagers[managerID]
	if conf.assets == "" {
		return false
	}
	_ = fs.CopyDirectory(
		filepath.Join(conf.assets, manager.ID),
		filepath.Join(conf.Path, manager.ID),
	)
	return true
}

// DownloadSchemeManager downloads and returns a scheme manager description.xml file
// from the specified URL.
func DownloadSchemeManager(url string) (*SchemeManager, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	if url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}
	if strings.HasSuffix(url, "/description.xml") {
		url = url[:len(url)-len("/description.xml")]
	}
	b, err := NewHTTPTransport(url).GetBytes("description.xml")
	if err != nil {
		return nil, err
	}
	manager := NewSchemeManager("")
	if err = xml.Unmarshal(b, manager); err != nil {
		return nil, err
	}

	manager.URL = url // TODO?
	return manager, nil
}

// RemoveSchemeManager removes the specified scheme manager and all associated issuers,
// public keys and credential types from this Configuration.
func (conf *Configuration) RemoveSchemeManager(id SchemeManagerIdentifier, fromStorage bool) error {
	// Remove everything falling under the manager's responsibility
	for credid := range conf.CredentialTypes {
		if credid.IssuerIdentifier().SchemeManagerIdentifier() == id {
			delete(conf.CredentialTypes, credid)
		}
	}
	for issid := range conf.Issuers {
		if issid.SchemeManagerIdentifier() == id {
			delete(conf.Issuers, issid)
		}
	}
	for issid := range conf.publicKeys {
		if issid.SchemeManagerIdentifier() == id {
			delete(conf.publicKeys, issid)
		}
	}
	delete(conf.SchemeManagers, id)

	if fromStorage {
		return os.RemoveAll(fmt.Sprintf("%s/%s", conf.Path, id.String()))
	}
	return nil
}

// InstallSchemeManager downloads and adds the specified scheme manager to this Configuration,
// provided its signature is valid.
func (conf *Configuration) InstallSchemeManager(manager *SchemeManager) error {
	name := manager.ID
	if err := fs.EnsureDirectoryExists(filepath.Join(conf.Path, name)); err != nil {
		return err
	}

	t := NewHTTPTransport(manager.URL)
	path := fmt.Sprintf("%s/%s", conf.Path, name)
	if err := t.GetFile("description.xml", path+"/description.xml"); err != nil {
		return err
	}
	if err := t.GetFile("pk.pem", path+"/pk.pem"); err != nil {
		return err
	}
	if err := conf.DownloadSchemeManagerSignature(manager); err != nil {
		return err
	}
	conf.SchemeManagers[manager.Identifier()] = manager
	if err := conf.UpdateSchemeManager(manager.Identifier(), nil); err != nil {
		return err
	}

	return conf.ParseSchemeManagerFolder(filepath.Join(conf.Path, name), manager)
}

// DownloadSchemeManagerSignature downloads, stores and verifies the latest version
// of the index file and signature of the specified manager.
func (conf *Configuration) DownloadSchemeManagerSignature(manager *SchemeManager) (err error) {
	t := NewHTTPTransport(manager.URL)
	path := fmt.Sprintf("%s/%s", conf.Path, manager.ID)
	index := filepath.Join(path, "index")
	sig := filepath.Join(path, "index.sig")

	// Backup so we can restore last valid signature if the new signature is invalid
	if err := conf.backupManagerSignature(index, sig); err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = conf.restoreManagerSignature(index, sig)
		}
	}()

	if err = t.GetFile("index", index); err != nil {
		return
	}
	if err = t.GetFile("index.sig", sig); err != nil {
		return
	}
	valid, err := conf.VerifySignature(manager.Identifier())
	if err != nil {
		return
	}
	if !valid {
		err = errors.New("Scheme manager signature invalid")
	}
	return
}

func (conf *Configuration) backupManagerSignature(index, sig string) error {
	if err := fs.Copy(index, index+".backup"); err != nil {
		return err
	}
	if err := fs.Copy(sig, sig+".backup"); err != nil {
		return err
	}
	return nil
}

func (conf *Configuration) restoreManagerSignature(index, sig string) error {
	if err := fs.Copy(index+".backup", index); err != nil {
		return err
	}
	if err := fs.Copy(sig+".backup", sig); err != nil {
		return err
	}
	return nil
}

// Download downloads the issuers, credential types and public keys specified in set
// if the current Configuration does not already have them,  and checks their authenticity
// using the scheme manager index.
func (conf *Configuration) Download(set *IrmaIdentifierSet) (downloaded *IrmaIdentifierSet, err error) {
	downloaded = &IrmaIdentifierSet{
		SchemeManagers:  map[SchemeManagerIdentifier]struct{}{},
		Issuers:         map[IssuerIdentifier]struct{}{},
		CredentialTypes: map[CredentialTypeIdentifier]struct{}{},
	}

	managers := make(map[SchemeManagerIdentifier]struct{})
	for issid := range set.Issuers {
		if _, contains := conf.Issuers[issid]; !contains {
			managers[issid.SchemeManagerIdentifier()] = struct{}{}
		}
	}
	for issid, keyids := range set.PublicKeys {
		for _, keyid := range keyids {
			pk, err := conf.PublicKey(issid, keyid)
			if err != nil {
				return nil, err
			}
			if pk == nil {
				managers[issid.SchemeManagerIdentifier()] = struct{}{}
			}
		}
	}
	for credid := range set.CredentialTypes {
		if _, contains := conf.CredentialTypes[credid]; !contains {
			managers[credid.IssuerIdentifier().SchemeManagerIdentifier()] = struct{}{}
		}
	}

	for id := range managers {
		if err = conf.UpdateSchemeManager(id, downloaded); err != nil {
			return
		}
	}

	if !downloaded.Empty() {
		return downloaded, conf.ParseFolder()
	}
	return
}

func (i SchemeManagerIndex) String() string {
	var paths []string
	var b bytes.Buffer

	for path := range i {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		b.WriteString(hex.EncodeToString(i[path]))
		b.WriteString(" ")
		b.WriteString(path)
		b.WriteString("\n")
	}

	return b.String()
}

// FromString populates this index by parsing the specified string.
func (i SchemeManagerIndex) FromString(s string) error {
	for j, line := range strings.Split(s, "\n") {
		if len(line) == 0 {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			return errors.Errorf("Scheme manager index line %d has incorrect amount of parts", j)
		}
		hash, err := hex.DecodeString(parts[0])
		if err != nil {
			return err
		}
		i[parts[1]] = hash
	}

	return nil
}

// parseIndex parses the index file of the specified manager.
func (conf *Configuration) parseIndex(name string, manager *SchemeManager) (SchemeManagerIndex, error) {
	path := filepath.Join(conf.Path, name, "index")
	if err := fs.AssertPathExists(path); err != nil {
		return nil, fmt.Errorf("Missing scheme manager index file; tried %s", path)
	}
	indexbts, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	index := SchemeManagerIndex(make(map[string]ConfigurationFileHash))
	return index, index.FromString(string(indexbts))
}

func (conf *Configuration) VerifySchemeManager(manager *SchemeManager) error {
	valid, err := conf.VerifySignature(manager.Identifier())
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("Scheme manager signature was invalid")
	}

	for file := range manager.index {
		exists, err := fs.PathExists(filepath.Join(conf.Path, file))
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		// Don't care about the actual bytes
		if _, _, err := conf.ReadAuthenticatedFile(manager, file); err != nil {
			return err
		}
	}

	return nil
}

// ReadAuthenticatedFile reads the file at the specified path
// and verifies its authenticity by checking that the file hash
// is present in the (signed) scheme manager index file.
func (conf *Configuration) ReadAuthenticatedFile(manager *SchemeManager, path string) ([]byte, bool, error) {
	signedHash, ok := manager.index[path]
	if !ok {
		return nil, false, nil
	}

	bts, err := ioutil.ReadFile(filepath.Join(conf.Path, path))
	if err != nil {
		return nil, true, err
	}
	computedHash := sha256.Sum256(bts)

	if !bytes.Equal(computedHash[:], signedHash) {
		return nil, true, errors.Errorf("Hash of %s does not match scheme manager index", path)
	}
	return bts, true, nil
}

// VerifySignature verifies the signature on the scheme manager index file
// (which contains the SHA256 hashes of all files under this scheme manager,
// which are used for verifying file authenticity).
func (conf *Configuration) VerifySignature(id SchemeManagerIdentifier) (valid bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			valid = false
			if e, ok := r.(error); ok {
				err = errors.Errorf("Scheme manager index signature failed to verify: %s", e.Error())
			} else {
				err = errors.New("Scheme manager index signature failed to verify")
			}
		}
	}()

	dir := filepath.Join(conf.Path, id.String())
	if err := fs.AssertPathExists(dir+"/index", dir+"/index.sig", dir+"/pk.pem"); err != nil {
		return false, errors.New("Missing scheme manager index file, signature, or public key")
	}

	// Read and hash index file
	indexbts, err := ioutil.ReadFile(dir + "/index")
	if err != nil {
		return false, err
	}
	indexhash := sha256.Sum256(indexbts)

	// Read and parse scheme manager public key
	pkbts, err := ioutil.ReadFile(dir + "/pk.pem")
	if err != nil {
		return false, err
	}
	pkblk, _ := pem.Decode(pkbts)
	genericPk, err := x509.ParsePKIXPublicKey(pkblk.Bytes)
	if err != nil {
		return false, err
	}
	pk, ok := genericPk.(*ecdsa.PublicKey)
	if !ok {
		return false, errors.New("Invalid scheme manager public key")
	}

	// Read and parse signature
	sig, err := ioutil.ReadFile(dir + "/index.sig")
	if err != nil {
		return false, err
	}
	ints := make([]*big.Int, 0, 2)
	_, err = asn1.Unmarshal(sig, &ints)

	// Verify signature
	return ecdsa.Verify(pk, indexhash[:], ints[0], ints[1]), nil
}

func (hash ConfigurationFileHash) String() string {
	return hex.EncodeToString(hash)
}

func (hash ConfigurationFileHash) Equal(other ConfigurationFileHash) bool {
	return bytes.Equal(hash, other)
}

// UpdateSchemeManager syncs the stored version within the irma_configuration directory
// with the remote version at the scheme manager's URL, downloading and storing
// new and modified files, according to the index files of both versions.
// It stores the identifiers of new or updated credential types or issuers in the second parameter.
// Note: any newly downloaded files are not yet parsed and inserted into conf.
func (conf *Configuration) UpdateSchemeManager(id SchemeManagerIdentifier, downloaded *IrmaIdentifierSet) (err error) {
	manager, contains := conf.SchemeManagers[id]
	if !contains {
		return errors.Errorf("Cannot update unknown scheme manager %s", id)
	}

	// Download the new index and its signature, and check that the new index
	// is validly signed by the new signature
	// By aborting immediately in case of error, and restoring backup versions
	// of the index and signature, we leave our stored copy of the scheme manager
	// intact.
	if err = conf.DownloadSchemeManagerSignature(manager); err != nil {
		return
	}
	newIndex, err := conf.parseIndex(manager.ID, manager)
	if err != nil {
		return
	}

	issPattern := regexp.MustCompile("(.+)/(.+)/description\\.xml")
	credPattern := regexp.MustCompile("(.+)/(.+)/Issues/(.+)/description\\.xml")
	transport := NewHTTPTransport("")

	// TODO: how to recover/fix local copy if err != nil below?
	for filename, newHash := range newIndex {
		path := filepath.Join(conf.Path, filename)
		oldHash, known := manager.index[filename]
		var have bool
		have, err = fs.PathExists(path)
		if err != nil {
			return err
		}
		if known && have && oldHash.Equal(newHash) {
			continue // nothing to do, we already have this file
		}
		// Ensure that the folder in which to write the file exists
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		stripped := filename[len(manager.ID)+1:] // Scheme manager URL already ends with its name
		// Download the new file, store it in our own irma_configuration folder
		if err = transport.GetFile(manager.URL+"/"+stripped, path); err != nil {
			return
		}
		// See if the file is a credential type or issuer, and add it to the downloaded set if so
		if downloaded == nil {
			continue
		}
		var matches []string
		matches = issPattern.FindStringSubmatch(filename)
		if len(matches) == 3 {
			issid := NewIssuerIdentifier(fmt.Sprintf("%s.%s", matches[1], matches[2]))
			downloaded.Issuers[issid] = struct{}{}
		}
		matches = credPattern.FindStringSubmatch(filename)
		if len(matches) == 4 {
			credid := NewCredentialTypeIdentifier(fmt.Sprintf("%s.%s.%s", matches[1], matches[2], matches[3]))
			downloaded.CredentialTypes[credid] = struct{}{}
		}
	}

	manager.index = newIndex
	return
}
