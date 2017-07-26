package irmago

import "strings"

type objectIdentifier string

// SchemeManagerIdentifier identifies a scheme manager. Equal to its ID. For example "irma-demo".
type SchemeManagerIdentifier struct {
	objectIdentifier
}

// IssuerIdentifier identifies an inssuer. For example "irma-demo.RU".
type IssuerIdentifier struct {
	objectIdentifier
}

// CredentialTypeIdentifier identifies a credentialtype. For example "irma-demo.RU.studentCard".
type CredentialTypeIdentifier struct {
	objectIdentifier
}

// AttributeIdentifier identifies an attribute. For example "irma-demo.RU.studentCard.studentID".
type AttributeIdentifier struct {
	objectIdentifier
}

func (oi objectIdentifier) Parent() string {
	str := string(oi)
	return str[:strings.LastIndex(str, "/")]
}

func (oi objectIdentifier) Name() string {
	str := string(oi)
	return str[strings.LastIndex(str, "/")+1:]
}

func (oi objectIdentifier) String() string {
	return string(oi)
}

// NewSchemeManagerIdentifier converts the specified identifier to a SchemeManagerIdentifier.
func NewSchemeManagerIdentifier(id string) SchemeManagerIdentifier {
	return SchemeManagerIdentifier{objectIdentifier(id)}
}

// NewIssuerIdentifier converts the specified identifier to a IssuerIdentifier.
func NewIssuerIdentifier(id string) IssuerIdentifier {
	return IssuerIdentifier{objectIdentifier(id)}
}

// NewCredentialTypeIdentifier converts the specified identifier to a CredentialTypeIdentifier.
func NewCredentialTypeIdentifier(id string) CredentialTypeIdentifier {
	return CredentialTypeIdentifier{objectIdentifier(id)}
}

// NewAttributeIdentifier converts the specified identifier to a AttributeIdentifier.
func NewAttributeIdentifier(id string) AttributeIdentifier {
	return AttributeIdentifier{objectIdentifier(id)}
}

// SchemeManagerIdentifier returns the scheme manager identifer of the issuer.
func (id IssuerIdentifier) SchemeManagerIdentifier() SchemeManagerIdentifier {
	return NewSchemeManagerIdentifier(id.Parent())
}

// IssuerIdentifier returns the IssuerIdentifier of the credential identifier.
func (id CredentialTypeIdentifier) IssuerIdentifier() IssuerIdentifier {
	return NewIssuerIdentifier(id.Parent())
}

// CredentialTypeIdentifier returns the CredentialTypeIdentifier of the attribute identifier.
func (id AttributeIdentifier) CredentialTypeIdentifier() CredentialTypeIdentifier {
	return NewCredentialTypeIdentifier(id.Parent())
}