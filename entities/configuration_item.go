package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// ConfigurationItem represents an Autotask ConfigurationItem (CI) entity.
//
// It is a root entity: create one with autotask.Create, which needs ProductID,
// IsActive and CompanyID per the field metadata (CompanyID is read-only after
// create), and change one with autotask.Update, which sends PATCH to
// /v1.0/ConfigurationItems per the REST swagger. Update sends every field that
// is set, and many fields are read-only per the field metadata (for example
// the Datto, RMM and SSL fields), so to change a few fields send a value that
// carries ID and only those fields. The API has no delete (canDelete is false
// in the entity metadata, and the swagger lists no DELETE); use IsActive to
// retire a configuration item.
//
// Its child entities are reached through the parent URL with the child calls:
// ConfigurationItemNote (ListChild, CreateChild, UpdateChild),
// ConfigurationItemAttachment, ConfigurationItemRelatedItem (ListChild,
// CreateChild, DeleteChild), ConfigurationItemBillingProductAssociation
// (ListChild, CreateChild, UpdateChild, DeleteChild), ConfigurationItemDnsRecord
// and ConfigurationItemSslSubjectAlternativeName (ListChild, DeleteChild). The
// notes' attachments hang off the note, see ConfigurationItemNoteAttachment.
//
// This file is hand-written. Its fields follow the entity field metadata of a
// live tenant (captured 2026-11-23); it was not generated, so do not
// regenerate it.
type ConfigurationItem struct {
	ApiVendorID                           autotask.Optional[int64]     `json:"apiVendorID,omitzero"`
	CompanyID                             autotask.Optional[int64]     `json:"companyID,omitzero"`
	CompanyLocationID                     autotask.Optional[int64]     `json:"companyLocationID,omitzero"`
	ConfigurationItemCategoryID           autotask.Optional[int64]     `json:"configurationItemCategoryID,omitzero"`
	ConfigurationItemType                 autotask.Optional[int64]     `json:"configurationItemType,omitzero"`
	ContactID                             autotask.Optional[int64]     `json:"contactID,omitzero"`
	ContractID                            autotask.Optional[int64]     `json:"contractID,omitzero"`
	ContractServiceBundleID               autotask.Optional[int64]     `json:"contractServiceBundleID,omitzero"`
	ContractServiceID                     autotask.Optional[int64]     `json:"contractServiceID,omitzero"`
	CreateDate                            autotask.Optional[time.Time] `json:"createDate,omitzero"`
	CreatedByPersonID                     autotask.Optional[int64]     `json:"createdByPersonID,omitzero"`
	DailyCost                             autotask.Optional[float64]   `json:"dailyCost,omitzero"`
	DattoAvailableKilobytes               autotask.Optional[int64]     `json:"dattoAvailableKilobytes,omitzero"`
	DattoDeviceMemoryMegabytes            autotask.Optional[int64]     `json:"dattoDeviceMemoryMegabytes,omitzero"`
	DattoDrivesErrors                     autotask.Optional[bool]      `json:"dattoDrivesErrors,omitzero"`
	DattoHostname                         autotask.Optional[string]    `json:"dattoHostname,omitzero"`
	DattoInternalIP                       autotask.Optional[string]    `json:"dattoInternalIP,omitzero"`
	DattoKernelVersionID                  autotask.Optional[int64]     `json:"dattoKernelVersionID,omitzero"`
	DattoLastCheckInDateTime              autotask.Optional[time.Time] `json:"dattoLastCheckInDateTime,omitzero"`
	DattoNicSpeedKilobitsPerSecond        autotask.Optional[int64]     `json:"dattoNicSpeedKilobitsPerSecond,omitzero"`
	DattoNumberOfAgents                   autotask.Optional[int64]     `json:"dattoNumberOfAgents,omitzero"`
	DattoNumberOfDrives                   autotask.Optional[int64]     `json:"dattoNumberOfDrives,omitzero"`
	DattoNumberOfVolumes                  autotask.Optional[int64]     `json:"dattoNumberOfVolumes,omitzero"`
	DattoOffsiteUsedBytes                 autotask.Optional[int64]     `json:"dattoOffsiteUsedBytes,omitzero"`
	DattoOSVersionID                      autotask.Optional[int64]     `json:"dattoOSVersionID,omitzero"`
	DattoPercentageUsed                   autotask.Optional[float64]   `json:"dattoPercentageUsed,omitzero"`
	DattoProtectedKilobytes               autotask.Optional[int64]     `json:"dattoProtectedKilobytes,omitzero"`
	DattoRemoteIP                         autotask.Optional[string]    `json:"dattoRemoteIP,omitzero"`
	DattoSerialNumber                     autotask.Optional[string]    `json:"dattoSerialNumber,omitzero"`
	DattoUptimeSeconds                    autotask.Optional[int64]     `json:"dattoUptimeSeconds,omitzero"`
	DattoUsedKilobytes                    autotask.Optional[int64]     `json:"dattoUsedKilobytes,omitzero"`
	DattoZfsVersionID                     autotask.Optional[int64]     `json:"dattoZfsVersionID,omitzero"`
	DeviceNetworkingID                    autotask.Optional[string]    `json:"deviceNetworkingID,omitzero"`
	Domain                                autotask.Optional[string]    `json:"domain,omitzero"`
	DomainExpirationDateTime              autotask.Optional[time.Time] `json:"domainExpirationDateTime,omitzero"`
	DomainLastUpdatedDateTime             autotask.Optional[time.Time] `json:"domainLastUpdatedDateTime,omitzero"`
	DomainRegistrarID                     autotask.Optional[int64]     `json:"domainRegistrarID,omitzero"`
	DomainRegistrationDateTime            autotask.Optional[time.Time] `json:"domainRegistrationDateTime,omitzero"`
	HourlyCost                            autotask.Optional[float64]   `json:"hourlyCost,omitzero"`
	ID                                    autotask.Optional[int64]     `json:"id,omitzero"`
	ImpersonatorCreatorResourceID         autotask.Optional[int64]     `json:"impersonatorCreatorResourceID,omitzero"`
	InstallDate                           autotask.Optional[time.Time] `json:"installDate,omitzero"`
	InstalledByContactID                  autotask.Optional[int64]     `json:"installedByContactID,omitzero"`
	InstalledByID                         autotask.Optional[int64]     `json:"installedByID,omitzero"`
	IsActive                              autotask.Optional[bool]      `json:"isActive,omitzero"`
	LastActivityPersonID                  autotask.Optional[int64]     `json:"lastActivityPersonID,omitzero"`
	LastActivityPersonType                autotask.Optional[int64]     `json:"lastActivityPersonType,omitzero"`
	LastModifiedTime                      autotask.Optional[time.Time] `json:"lastModifiedTime,omitzero"`
	Location                              autotask.Optional[string]    `json:"location,omitzero"`
	MonthlyCost                           autotask.Optional[float64]   `json:"monthlyCost,omitzero"`
	Notes                                 autotask.Optional[string]    `json:"notes,omitzero"`
	NumberOfUsers                         autotask.Optional[float64]   `json:"numberOfUsers,omitzero"`
	ParentConfigurationItemID             autotask.Optional[int64]     `json:"parentConfigurationItemID,omitzero"`
	PerUseCost                            autotask.Optional[float64]   `json:"perUseCost,omitzero"`
	ProductID                             autotask.Optional[int64]     `json:"productID,omitzero"`
	ReferenceNumber                       autotask.Optional[string]    `json:"referenceNumber,omitzero"`
	ReferenceTitle                        autotask.Optional[string]    `json:"referenceTitle,omitzero"`
	RmmDeviceAuditAntivirusStatusID       autotask.Optional[int64]     `json:"rmmDeviceAuditAntivirusStatusID,omitzero"`
	RmmDeviceAuditArchitectureID          autotask.Optional[int64]     `json:"rmmDeviceAuditArchitectureID,omitzero"`
	RmmDeviceAuditBackupStatusID          autotask.Optional[int64]     `json:"rmmDeviceAuditBackupStatusID,omitzero"`
	RmmDeviceAuditDescription             autotask.Optional[string]    `json:"rmmDeviceAuditDescription,omitzero"`
	RmmDeviceAuditDeviceTypeID            autotask.Optional[int64]     `json:"rmmDeviceAuditDeviceTypeID,omitzero"`
	RmmDeviceAuditDisplayAdaptorID        autotask.Optional[int64]     `json:"rmmDeviceAuditDisplayAdaptorID,omitzero"`
	RmmDeviceAuditDomainID                autotask.Optional[int64]     `json:"rmmDeviceAuditDomainID,omitzero"`
	RmmDeviceAuditExternalIPAddress       autotask.Optional[string]    `json:"rmmDeviceAuditExternalIPAddress,omitzero"`
	RmmDeviceAuditHostname                autotask.Optional[string]    `json:"rmmDeviceAuditHostname,omitzero"`
	RmmDeviceAuditIPAddress               autotask.Optional[string]    `json:"rmmDeviceAuditIPAddress,omitzero"`
	RmmDeviceAuditLastUser                autotask.Optional[string]    `json:"rmmDeviceAuditLastUser,omitzero"`
	RmmDeviceAuditMacAddress              autotask.Optional[string]    `json:"rmmDeviceAuditMacAddress,omitzero"`
	RmmDeviceAuditManufacturerID          autotask.Optional[int64]     `json:"rmmDeviceAuditManufacturerID,omitzero"`
	RmmDeviceAuditMemoryBytes             autotask.Optional[int64]     `json:"rmmDeviceAuditMemoryBytes,omitzero"`
	RmmDeviceAuditMissingPatchCount       autotask.Optional[int64]     `json:"rmmDeviceAuditMissingPatchCount,omitzero"`
	RmmDeviceAuditMobileNetworkOperatorID autotask.Optional[int64]     `json:"rmmDeviceAuditMobileNetworkOperatorID,omitzero"`
	RmmDeviceAuditMobileNumber            autotask.Optional[string]    `json:"rmmDeviceAuditMobileNumber,omitzero"`
	RmmDeviceAuditModelID                 autotask.Optional[int64]     `json:"rmmDeviceAuditModelID,omitzero"`
	RmmDeviceAuditMotherboardID           autotask.Optional[int64]     `json:"rmmDeviceAuditMotherboardID,omitzero"`
	RmmDeviceAuditOperatingSystem         autotask.Optional[string]    `json:"rmmDeviceAuditOperatingSystem,omitzero"`
	RmmDeviceAuditPatchStatusID           autotask.Optional[int64]     `json:"rmmDeviceAuditPatchStatusID,omitzero"`
	RmmDeviceAuditProcessorID             autotask.Optional[int64]     `json:"rmmDeviceAuditProcessorID,omitzero"`
	RmmDeviceAuditServicePackID           autotask.Optional[int64]     `json:"rmmDeviceAuditServicePackID,omitzero"`
	RmmDeviceAuditSnmpContact             autotask.Optional[string]    `json:"rmmDeviceAuditSnmpContact,omitzero"`
	RmmDeviceAuditSnmpLocation            autotask.Optional[string]    `json:"rmmDeviceAuditSnmpLocation,omitzero"`
	RmmDeviceAuditSnmpName                autotask.Optional[string]    `json:"rmmDeviceAuditSnmpName,omitzero"`
	RmmDeviceAuditSoftwareStatusID        autotask.Optional[int64]     `json:"rmmDeviceAuditSoftwareStatusID,omitzero"`
	RmmDeviceAuditStorageBytes            autotask.Optional[int64]     `json:"rmmDeviceAuditStorageBytes,omitzero"`
	RmmDeviceID                           autotask.Optional[int64]     `json:"rmmDeviceID,omitzero"`
	RmmDeviceUid                          autotask.Optional[string]    `json:"rmmDeviceUid,omitzero"`
	RmmOpenAlertCount                     autotask.Optional[int64]     `json:"rmmOpenAlertCount,omitzero"`
	SerialNumber                          autotask.Optional[string]    `json:"serialNumber,omitzero"`
	ServiceBundleID                       autotask.Optional[int64]     `json:"serviceBundleID,omitzero"`
	ServiceID                             autotask.Optional[int64]     `json:"serviceID,omitzero"`
	ServiceLevelAgreementID               autotask.Optional[int64]     `json:"serviceLevelAgreementID,omitzero"`
	SetupFee                              autotask.Optional[float64]   `json:"setupFee,omitzero"`
	SourceChargeID                        autotask.Optional[int64]     `json:"sourceChargeID,omitzero"`
	SourceChargeType                      autotask.Optional[int64]     `json:"sourceChargeType,omitzero"`
	SslCommonName                         autotask.Optional[string]    `json:"sslCommonName,omitzero"`
	SslIssuedBy                           autotask.Optional[string]    `json:"sslIssuedBy,omitzero"`
	SslLocation                           autotask.Optional[string]    `json:"sslLocation,omitzero"`
	SslOrganization                       autotask.Optional[string]    `json:"sslOrganization,omitzero"`
	SslOrganizationUnit                   autotask.Optional[string]    `json:"sslOrganizationUnit,omitzero"`
	SslSerialNumber                       autotask.Optional[string]    `json:"sslSerialNumber,omitzero"`
	SslSignatureAlgorithm                 autotask.Optional[string]    `json:"sslSignatureAlgorithm,omitzero"`
	SslSource                             autotask.Optional[string]    `json:"sslSource,omitzero"`
	SslValidFromDateTime                  autotask.Optional[time.Time] `json:"sslValidFromDateTime,omitzero"`
	SslValidUntilDateTime                 autotask.Optional[time.Time] `json:"sslValidUntilDateTime,omitzero"`
	VendorID                              autotask.Optional[int64]     `json:"vendorID,omitzero"`
	WarrantyExpirationDate                autotask.Optional[time.Time] `json:"warrantyExpirationDate,omitzero"`
	// Deprecated: ConfigurationItems has no lastActivityDate field in the
	// Autotask entity metadata, so the API never returns it. Do not set it.
	LastActivityDate  autotask.Optional[time.Time] `json:"lastActivityDate,omitzero"`
	UserDefinedFields []autotask.UDF               `json:"userDefinedFields,omitempty"`
}

// EntityName returns the Autotask API entity name for configuration items.
func (ConfigurationItem) EntityName() string { return "ConfigurationItems" }
