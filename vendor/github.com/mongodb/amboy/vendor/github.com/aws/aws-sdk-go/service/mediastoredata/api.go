// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

package mediastoredata

import (
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
)

const opDeleteObject = "DeleteObject"

// DeleteObjectRequest generates a "aws/request.Request" representing the
// client's request for the DeleteObject operation. The "output" return
// value will be populated with the request's response once the request completes
// successfully.
//
// Use "Send" method on the returned Request to send the API call to the service.
// the "output" return value is not valid until after Send returns without error.
//
// See DeleteObject for more information on using the DeleteObject
// API call, and error handling.
//
// This method is useful when you want to inject custom logic or configuration
// into the SDK's request lifecycle. Such as custom headers, or retry logic.
//
//
//    // Example sending a request using the DeleteObjectRequest method.
//    req, resp := client.DeleteObjectRequest(params)
//
//    err := req.Send()
//    if err == nil { // resp is now filled
//        fmt.Println(resp)
//    }
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/mediastore-data-2017-09-01/DeleteObject
func (c *MediaStoreData) DeleteObjectRequest(input *DeleteObjectInput) (req *request.Request, output *DeleteObjectOutput) {
	op := &request.Operation{
		Name:       opDeleteObject,
		HTTPMethod: "DELETE",
		HTTPPath:   "/{Path+}",
	}

	if input == nil {
		input = &DeleteObjectInput{}
	}

	output = &DeleteObjectOutput{}
	req = c.newRequest(op, input, output)
	return
}

// DeleteObject API operation for AWS Elemental MediaStore Data Plane.
//
// Deletes an object at the specified path.
//
// Returns awserr.Error for service API and SDK errors. Use runtime type assertions
// with awserr.Error's Code and Message methods to get detailed information about
// the error.
//
// See the AWS API reference guide for AWS Elemental MediaStore Data Plane's
// API operation DeleteObject for usage and error information.
//
// Returned Error Codes:
//   * ErrCodeContainerNotFoundException "ContainerNotFoundException"
//   The specified container was not found for the specified account.
//
//   * ErrCodeObjectNotFoundException "ObjectNotFoundException"
//   Could not perform an operation on an object that does not exist.
//
//   * ErrCodeInternalServerError "InternalServerError"
//   The service is temporarily unavailable.
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/mediastore-data-2017-09-01/DeleteObject
func (c *MediaStoreData) DeleteObject(input *DeleteObjectInput) (*DeleteObjectOutput, error) {
	req, out := c.DeleteObjectRequest(input)
	return out, req.Send()
}

// DeleteObjectWithContext is the same as DeleteObject with the addition of
// the ability to pass a context and additional request options.
//
// See DeleteObject for details on how to use this API operation.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *MediaStoreData) DeleteObjectWithContext(ctx aws.Context, input *DeleteObjectInput, opts ...request.Option) (*DeleteObjectOutput, error) {
	req, out := c.DeleteObjectRequest(input)
	req.SetContext(ctx)
	req.ApplyOptions(opts...)
	return out, req.Send()
}

const opDescribeObject = "DescribeObject"

// DescribeObjectRequest generates a "aws/request.Request" representing the
// client's request for the DescribeObject operation. The "output" return
// value will be populated with the request's response once the request completes
// successfully.
//
// Use "Send" method on the returned Request to send the API call to the service.
// the "output" return value is not valid until after Send returns without error.
//
// See DescribeObject for more information on using the DescribeObject
// API call, and error handling.
//
// This method is useful when you want to inject custom logic or configuration
// into the SDK's request lifecycle. Such as custom headers, or retry logic.
//
//
//    // Example sending a request using the DescribeObjectRequest method.
//    req, resp := client.DescribeObjectRequest(params)
//
//    err := req.Send()
//    if err == nil { // resp is now filled
//        fmt.Println(resp)
//    }
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/mediastore-data-2017-09-01/DescribeObject
func (c *MediaStoreData) DescribeObjectRequest(input *DescribeObjectInput) (req *request.Request, output *DescribeObjectOutput) {
	op := &request.Operation{
		Name:       opDescribeObject,
		HTTPMethod: "HEAD",
		HTTPPath:   "/{Path+}",
	}

	if input == nil {
		input = &DescribeObjectInput{}
	}

	output = &DescribeObjectOutput{}
	req = c.newRequest(op, input, output)
	return
}

// DescribeObject API operation for AWS Elemental MediaStore Data Plane.
//
// Gets the headers for an object at the specified path.
//
// Returns awserr.Error for service API and SDK errors. Use runtime type assertions
// with awserr.Error's Code and Message methods to get detailed information about
// the error.
//
// See the AWS API reference guide for AWS Elemental MediaStore Data Plane's
// API operation DescribeObject for usage and error information.
//
// Returned Error Codes:
//   * ErrCodeContainerNotFoundException "ContainerNotFoundException"
//   The specified container was not found for the specified account.
//
//   * ErrCodeObjectNotFoundException "ObjectNotFoundException"
//   Could not perform an operation on an object that does not exist.
//
//   * ErrCodeInternalServerError "InternalServerError"
//   The service is temporarily unavailable.
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/mediastore-data-2017-09-01/DescribeObject
func (c *MediaStoreData) DescribeObject(input *DescribeObjectInput) (*DescribeObjectOutput, error) {
	req, out := c.DescribeObjectRequest(input)
	return out, req.Send()
}

// DescribeObjectWithContext is the same as DescribeObject with the addition of
// the ability to pass a context and additional request options.
//
// See DescribeObject for details on how to use this API operation.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *MediaStoreData) DescribeObjectWithContext(ctx aws.Context, input *DescribeObjectInput, opts ...request.Option) (*DescribeObjectOutput, error) {
	req, out := c.DescribeObjectRequest(input)
	req.SetContext(ctx)
	req.ApplyOptions(opts...)
	return out, req.Send()
}

const opGetObject = "GetObject"

// GetObjectRequest generates a "aws/request.Request" representing the
// client's request for the GetObject operation. The "output" return
// value will be populated with the request's response once the request completes
// successfully.
//
// Use "Send" method on the returned Request to send the API call to the service.
// the "output" return value is not valid until after Send returns without error.
//
// See GetObject for more information on using the GetObject
// API call, and error handling.
//
// This method is useful when you want to inject custom logic or configuration
// into the SDK's request lifecycle. Such as custom headers, or retry logic.
//
//
//    // Example sending a request using the GetObjectRequest method.
//    req, resp := client.GetObjectRequest(params)
//
//    err := req.Send()
//    if err == nil { // resp is now filled
//        fmt.Println(resp)
//    }
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/mediastore-data-2017-09-01/GetObject
func (c *MediaStoreData) GetObjectRequest(input *GetObjectInput) (req *request.Request, output *GetObjectOutput) {
	op := &request.Operation{
		Name:       opGetObject,
		HTTPMethod: "GET",
		HTTPPath:   "/{Path+}",
	}

	if input == nil {
		input = &GetObjectInput{}
	}

	output = &GetObjectOutput{}
	req = c.newRequest(op, input, output)
	return
}

// GetObject API operation for AWS Elemental MediaStore Data Plane.
//
// Downloads the object at the specified path.
//
// Returns awserr.Error for service API and SDK errors. Use runtime type assertions
// with awserr.Error's Code and Message methods to get detailed information about
// the error.
//
// See the AWS API reference guide for AWS Elemental MediaStore Data Plane's
// API operation GetObject for usage and error information.
//
// Returned Error Codes:
//   * ErrCodeContainerNotFoundException "ContainerNotFoundException"
//   The specified container was not found for the specified account.
//
//   * ErrCodeObjectNotFoundException "ObjectNotFoundException"
//   Could not perform an operation on an object that does not exist.
//
//   * ErrCodeRequestedRangeNotSatisfiableException "RequestedRangeNotSatisfiableException"
//   The requested content range is not valid.
//
//   * ErrCodeInternalServerError "InternalServerError"
//   The service is temporarily unavailable.
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/mediastore-data-2017-09-01/GetObject
func (c *MediaStoreData) GetObject(input *GetObjectInput) (*GetObjectOutput, error) {
	req, out := c.GetObjectRequest(input)
	return out, req.Send()
}

// GetObjectWithContext is the same as GetObject with the addition of
// the ability to pass a context and additional request options.
//
// See GetObject for details on how to use this API operation.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *MediaStoreData) GetObjectWithContext(ctx aws.Context, input *GetObjectInput, opts ...request.Option) (*GetObjectOutput, error) {
	req, out := c.GetObjectRequest(input)
	req.SetContext(ctx)
	req.ApplyOptions(opts...)
	return out, req.Send()
}

const opListItems = "ListItems"

// ListItemsRequest generates a "aws/request.Request" representing the
// client's request for the ListItems operation. The "output" return
// value will be populated with the request's response once the request completes
// successfully.
//
// Use "Send" method on the returned Request to send the API call to the service.
// the "output" return value is not valid until after Send returns without error.
//
// See ListItems for more information on using the ListItems
// API call, and error handling.
//
// This method is useful when you want to inject custom logic or configuration
// into the SDK's request lifecycle. Such as custom headers, or retry logic.
//
//
//    // Example sending a request using the ListItemsRequest method.
//    req, resp := client.ListItemsRequest(params)
//
//    err := req.Send()
//    if err == nil { // resp is now filled
//        fmt.Println(resp)
//    }
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/mediastore-data-2017-09-01/ListItems
func (c *MediaStoreData) ListItemsRequest(input *ListItemsInput) (req *request.Request, output *ListItemsOutput) {
	op := &request.Operation{
		Name:       opListItems,
		HTTPMethod: "GET",
		HTTPPath:   "/",
	}

	if input == nil {
		input = &ListItemsInput{}
	}

	output = &ListItemsOutput{}
	req = c.newRequest(op, input, output)
	return
}

// ListItems API operation for AWS Elemental MediaStore Data Plane.
//
// Provides a list of metadata entries about folders and objects in the specified
// folder.
//
// Returns awserr.Error for service API and SDK errors. Use runtime type assertions
// with awserr.Error's Code and Message methods to get detailed information about
// the error.
//
// See the AWS API reference guide for AWS Elemental MediaStore Data Plane's
// API operation ListItems for usage and error information.
//
// Returned Error Codes:
//   * ErrCodeContainerNotFoundException "ContainerNotFoundException"
//   The specified container was not found for the specified account.
//
//   * ErrCodeInternalServerError "InternalServerError"
//   The service is temporarily unavailable.
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/mediastore-data-2017-09-01/ListItems
func (c *MediaStoreData) ListItems(input *ListItemsInput) (*ListItemsOutput, error) {
	req, out := c.ListItemsRequest(input)
	return out, req.Send()
}

// ListItemsWithContext is the same as ListItems with the addition of
// the ability to pass a context and additional request options.
//
// See ListItems for details on how to use this API operation.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *MediaStoreData) ListItemsWithContext(ctx aws.Context, input *ListItemsInput, opts ...request.Option) (*ListItemsOutput, error) {
	req, out := c.ListItemsRequest(input)
	req.SetContext(ctx)
	req.ApplyOptions(opts...)
	return out, req.Send()
}

const opPutObject = "PutObject"

// PutObjectRequest generates a "aws/request.Request" representing the
// client's request for the PutObject operation. The "output" return
// value will be populated with the request's response once the request completes
// successfully.
//
// Use "Send" method on the returned Request to send the API call to the service.
// the "output" return value is not valid until after Send returns without error.
//
// See PutObject for more information on using the PutObject
// API call, and error handling.
//
// This method is useful when you want to inject custom logic or configuration
// into the SDK's request lifecycle. Such as custom headers, or retry logic.
//
//
//    // Example sending a request using the PutObjectRequest method.
//    req, resp := client.PutObjectRequest(params)
//
//    err := req.Send()
//    if err == nil { // resp is now filled
//        fmt.Println(resp)
//    }
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/mediastore-data-2017-09-01/PutObject
func (c *MediaStoreData) PutObjectRequest(input *PutObjectInput) (req *request.Request, output *PutObjectOutput) {
	op := &request.Operation{
		Name:       opPutObject,
		HTTPMethod: "PUT",
		HTTPPath:   "/{Path+}",
	}

	if input == nil {
		input = &PutObjectInput{}
	}

	output = &PutObjectOutput{}
	req = c.newRequest(op, input, output)
	req.Handlers.Sign.Remove(v4.SignRequestHandler)
	handler := v4.BuildNamedHandler("v4.CustomSignerHandler", v4.WithUnsignedPayload)
	req.Handlers.Sign.PushFrontNamed(handler)
	return
}

// PutObject API operation for AWS Elemental MediaStore Data Plane.
//
// Uploads an object to the specified path. Object sizes are limited to 10 MB.
//
// Returns awserr.Error for service API and SDK errors. Use runtime type assertions
// with awserr.Error's Code and Message methods to get detailed information about
// the error.
//
// See the AWS API reference guide for AWS Elemental MediaStore Data Plane's
// API operation PutObject for usage and error information.
//
// Returned Error Codes:
//   * ErrCodeContainerNotFoundException "ContainerNotFoundException"
//   The specified container was not found for the specified account.
//
//   * ErrCodeInternalServerError "InternalServerError"
//   The service is temporarily unavailable.
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/mediastore-data-2017-09-01/PutObject
func (c *MediaStoreData) PutObject(input *PutObjectInput) (*PutObjectOutput, error) {
	req, out := c.PutObjectRequest(input)
	return out, req.Send()
}

// PutObjectWithContext is the same as PutObject with the addition of
// the ability to pass a context and additional request options.
//
// See PutObject for details on how to use this API operation.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *MediaStoreData) PutObjectWithContext(ctx aws.Context, input *PutObjectInput, opts ...request.Option) (*PutObjectOutput, error) {
	req, out := c.PutObjectRequest(input)
	req.SetContext(ctx)
	req.ApplyOptions(opts...)
	return out, req.Send()
}

type DeleteObjectInput struct {
	_ struct{} `type:"structure"`

	// The path (including the file name) where the object is stored in the container.
	// Format: <folder name>/<folder name>/<file name>
	//
	// Path is a required field
	Path *string `location:"uri" locationName:"Path" min:"1" type:"string" required:"true"`
}

// String returns the string representation
func (s DeleteObjectInput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s DeleteObjectInput) GoString() string {
	return s.String()
}

// Validate inspects the fields of the type to determine if they are valid.
func (s *DeleteObjectInput) Validate() error {
	invalidParams := request.ErrInvalidParams{Context: "DeleteObjectInput"}
	if s.Path == nil {
		invalidParams.Add(request.NewErrParamRequired("Path"))
	}
	if s.Path != nil && len(*s.Path) < 1 {
		invalidParams.Add(request.NewErrParamMinLen("Path", 1))
	}

	if invalidParams.Len() > 0 {
		return invalidParams
	}
	return nil
}

// SetPath sets the Path field's value.
func (s *DeleteObjectInput) SetPath(v string) *DeleteObjectInput {
	s.Path = &v
	return s
}

type DeleteObjectOutput struct {
	_ struct{} `type:"structure"`
}

// String returns the string representation
func (s DeleteObjectOutput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s DeleteObjectOutput) GoString() string {
	return s.String()
}

type DescribeObjectInput struct {
	_ struct{} `type:"structure"`

	// The path (including the file name) where the object is stored in the container.
	// Format: <folder name>/<folder name>/<file name>
	//
	// Path is a required field
	Path *string `location:"uri" locationName:"Path" min:"1" type:"string" required:"true"`
}

// String returns the string representation
func (s DescribeObjectInput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s DescribeObjectInput) GoString() string {
	return s.String()
}

// Validate inspects the fields of the type to determine if they are valid.
func (s *DescribeObjectInput) Validate() error {
	invalidParams := request.ErrInvalidParams{Context: "DescribeObjectInput"}
	if s.Path == nil {
		invalidParams.Add(request.NewErrParamRequired("Path"))
	}
	if s.Path != nil && len(*s.Path) < 1 {
		invalidParams.Add(request.NewErrParamMinLen("Path", 1))
	}

	if invalidParams.Len() > 0 {
		return invalidParams
	}
	return nil
}

// SetPath sets the Path field's value.
func (s *DescribeObjectInput) SetPath(v string) *DescribeObjectInput {
	s.Path = &v
	return s
}

type DescribeObjectOutput struct {
	_ struct{} `type:"structure"`

	// An optional CacheControl header that allows the caller to control the object's
	// cache behavior. Headers can be passed in as specified in the HTTP at https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9
	// (https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9).
	//
	// Headers with a custom user-defined value are also accepted.
	CacheControl *string `location:"header" locationName:"Cache-Control" type:"string"`

	// The length of the object in bytes.
	ContentLength *int64 `location:"header" locationName:"Content-Length" type:"long"`

	// The content type of the object.
	ContentType *string `location:"header" locationName:"Content-Type" type:"string"`

	// The ETag that represents a unique instance of the object.
	ETag *string `location:"header" locationName:"ETag" min:"1" type:"string"`

	// The date and time that the object was last modified.
	LastModified *time.Time `location:"header" locationName:"Last-Modified" type:"timestamp"`
}

// String returns the string representation
func (s DescribeObjectOutput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s DescribeObjectOutput) GoString() string {
	return s.String()
}

// SetCacheControl sets the CacheControl field's value.
func (s *DescribeObjectOutput) SetCacheControl(v string) *DescribeObjectOutput {
	s.CacheControl = &v
	return s
}

// SetContentLength sets the ContentLength field's value.
func (s *DescribeObjectOutput) SetContentLength(v int64) *DescribeObjectOutput {
	s.ContentLength = &v
	return s
}

// SetContentType sets the ContentType field's value.
func (s *DescribeObjectOutput) SetContentType(v string) *DescribeObjectOutput {
	s.ContentType = &v
	return s
}

// SetETag sets the ETag field's value.
func (s *DescribeObjectOutput) SetETag(v string) *DescribeObjectOutput {
	s.ETag = &v
	return s
}

// SetLastModified sets the LastModified field's value.
func (s *DescribeObjectOutput) SetLastModified(v time.Time) *DescribeObjectOutput {
	s.LastModified = &v
	return s
}

type GetObjectInput struct {
	_ struct{} `type:"structure"`

	// The path (including the file name) where the object is stored in the container.
	// Format: <folder name>/<folder name>/<file name>
	//
	// For example, to upload the file mlaw.avi to the folder path premium\canada
	// in the container movies, enter the path premium/canada/mlaw.avi.
	//
	// Do not include the container name in this path.
	//
	// If the path includes any folders that don't exist yet, the service creates
	// them. For example, suppose you have an existing premium/usa subfolder. If
	// you specify premium/canada, the service creates a canada subfolder in the
	// premium folder. You then have two subfolders, usa and canada, in the premium
	// folder.
	//
	// There is no correlation between the path to the source and the path (folders)
	// in the container in AWS Elemental MediaStore.
	//
	// For more information about folders and how they exist in a container, see
	// the AWS Elemental MediaStore User Guide (http://docs.aws.amazon.com/mediastore/latest/ug/).
	//
	// The file name is the name that is assigned to the file that you upload. The
	// file can have the same name inside and outside of AWS Elemental MediaStore,
	// or it can have the same name. The file name can include or omit an extension.
	//
	// Path is a required field
	Path *string `location:"uri" locationName:"Path" min:"1" type:"string" required:"true"`

	// The range bytes of an object to retrieve. For more information about the
	// Range header, go to http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.35
	// (http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.35).
	Range *string `location:"header" locationName:"Range" type:"string"`
}

// String returns the string representation
func (s GetObjectInput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s GetObjectInput) GoString() string {
	return s.String()
}

// Validate inspects the fields of the type to determine if they are valid.
func (s *GetObjectInput) Validate() error {
	invalidParams := request.ErrInvalidParams{Context: "GetObjectInput"}
	if s.Path == nil {
		invalidParams.Add(request.NewErrParamRequired("Path"))
	}
	if s.Path != nil && len(*s.Path) < 1 {
		invalidParams.Add(request.NewErrParamMinLen("Path", 1))
	}

	if invalidParams.Len() > 0 {
		return invalidParams
	}
	return nil
}

// SetPath sets the Path field's value.
func (s *GetObjectInput) SetPath(v string) *GetObjectInput {
	s.Path = &v
	return s
}

// SetRange sets the Range field's value.
func (s *GetObjectInput) SetRange(v string) *GetObjectInput {
	s.Range = &v
	return s
}

type GetObjectOutput struct {
	_ struct{} `type:"structure" payload:"Body"`

	// The bytes of the object.
	Body io.ReadCloser `type:"blob"`

	// An optional CacheControl header that allows the caller to control the object's
	// cache behavior. Headers can be passed in as specified in the HTTP spec at
	// https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9 (https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9).
	//
	// Headers with a custom user-defined value are also accepted.
	CacheControl *string `location:"header" locationName:"Cache-Control" type:"string"`

	// The length of the object in bytes.
	ContentLength *int64 `location:"header" locationName:"Content-Length" type:"long"`

	// The range of bytes to retrieve.
	ContentRange *string `location:"header" locationName:"Content-Range" type:"string"`

	// The content type of the object.
	ContentType *string `location:"header" locationName:"Content-Type" type:"string"`

	// The ETag that represents a unique instance of the object.
	ETag *string `location:"header" locationName:"ETag" min:"1" type:"string"`

	// The date and time that the object was last modified.
	LastModified *time.Time `location:"header" locationName:"Last-Modified" type:"timestamp"`

	// The HTML status code of the request. Status codes ranging from 200 to 299
	// indicate success. All other status codes indicate the type of error that
	// occurred.
	//
	// StatusCode is a required field
	StatusCode *int64 `location:"statusCode" type:"integer" required:"true"`
}

// String returns the string representation
func (s GetObjectOutput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s GetObjectOutput) GoString() string {
	return s.String()
}

// SetBody sets the Body field's value.
func (s *GetObjectOutput) SetBody(v io.ReadCloser) *GetObjectOutput {
	s.Body = v
	return s
}

// SetCacheControl sets the CacheControl field's value.
func (s *GetObjectOutput) SetCacheControl(v string) *GetObjectOutput {
	s.CacheControl = &v
	return s
}

// SetContentLength sets the ContentLength field's value.
func (s *GetObjectOutput) SetContentLength(v int64) *GetObjectOutput {
	s.ContentLength = &v
	return s
}

// SetContentRange sets the ContentRange field's value.
func (s *GetObjectOutput) SetContentRange(v string) *GetObjectOutput {
	s.ContentRange = &v
	return s
}

// SetContentType sets the ContentType field's value.
func (s *GetObjectOutput) SetContentType(v string) *GetObjectOutput {
	s.ContentType = &v
	return s
}

// SetETag sets the ETag field's value.
func (s *GetObjectOutput) SetETag(v string) *GetObjectOutput {
	s.ETag = &v
	return s
}

// SetLastModified sets the LastModified field's value.
func (s *GetObjectOutput) SetLastModified(v time.Time) *GetObjectOutput {
	s.LastModified = &v
	return s
}

// SetStatusCode sets the StatusCode field's value.
func (s *GetObjectOutput) SetStatusCode(v int64) *GetObjectOutput {
	s.StatusCode = &v
	return s
}

// A metadata entry for a folder or object.
type Item struct {
	_ struct{} `type:"structure"`

	// The length of the item in bytes.
	ContentLength *int64 `type:"long"`

	// The content type of the item.
	ContentType *string `type:"string"`

	// The ETag that represents a unique instance of the item.
	ETag *string `min:"1" type:"string"`

	// The date and time that the item was last modified.
	LastModified *time.Time `type:"timestamp"`

	// The name of the item.
	Name *string `type:"string"`

	// The item type (folder or object).
	Type *string `type:"string" enum:"ItemType"`
}

// String returns the string representation
func (s Item) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s Item) GoString() string {
	return s.String()
}

// SetContentLength sets the ContentLength field's value.
func (s *Item) SetContentLength(v int64) *Item {
	s.ContentLength = &v
	return s
}

// SetContentType sets the ContentType field's value.
func (s *Item) SetContentType(v string) *Item {
	s.ContentType = &v
	return s
}

// SetETag sets the ETag field's value.
func (s *Item) SetETag(v string) *Item {
	s.ETag = &v
	return s
}

// SetLastModified sets the LastModified field's value.
func (s *Item) SetLastModified(v time.Time) *Item {
	s.LastModified = &v
	return s
}

// SetName sets the Name field's value.
func (s *Item) SetName(v string) *Item {
	s.Name = &v
	return s
}

// SetType sets the Type field's value.
func (s *Item) SetType(v string) *Item {
	s.Type = &v
	return s
}

type ListItemsInput struct {
	_ struct{} `type:"structure"`

	// The maximum results to return. The service might return fewer results.
	MaxResults *int64 `location:"querystring" locationName:"MaxResults" min:"1" type:"integer"`

	// The NextToken received in the ListItemsResponse for the same container and
	// path. Tokens expire after 15 minutes.
	NextToken *string `location:"querystring" locationName:"NextToken" type:"string"`

	// The path in the container from which to retrieve items. Format: <folder name>/<folder
	// name>/<file name>
	Path *string `location:"querystring" locationName:"Path" type:"string"`
}

// String returns the string representation
func (s ListItemsInput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s ListItemsInput) GoString() string {
	return s.String()
}

// Validate inspects the fields of the type to determine if they are valid.
func (s *ListItemsInput) Validate() error {
	invalidParams := request.ErrInvalidParams{Context: "ListItemsInput"}
	if s.MaxResults != nil && *s.MaxResults < 1 {
		invalidParams.Add(request.NewErrParamMinValue("MaxResults", 1))
	}

	if invalidParams.Len() > 0 {
		return invalidParams
	}
	return nil
}

// SetMaxResults sets the MaxResults field's value.
func (s *ListItemsInput) SetMaxResults(v int64) *ListItemsInput {
	s.MaxResults = &v
	return s
}

// SetNextToken sets the NextToken field's value.
func (s *ListItemsInput) SetNextToken(v string) *ListItemsInput {
	s.NextToken = &v
	return s
}

// SetPath sets the Path field's value.
func (s *ListItemsInput) SetPath(v string) *ListItemsInput {
	s.Path = &v
	return s
}

type ListItemsOutput struct {
	_ struct{} `type:"structure"`

	// Metadata entries for the folders and objects at the requested path.
	Items []*Item `type:"list"`

	// The NextToken used to request the next page of results using ListItems.
	NextToken *string `type:"string"`
}

// String returns the string representation
func (s ListItemsOutput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s ListItemsOutput) GoString() string {
	return s.String()
}

// SetItems sets the Items field's value.
func (s *ListItemsOutput) SetItems(v []*Item) *ListItemsOutput {
	s.Items = v
	return s
}

// SetNextToken sets the NextToken field's value.
func (s *ListItemsOutput) SetNextToken(v string) *ListItemsOutput {
	s.NextToken = &v
	return s
}

type PutObjectInput struct {
	_ struct{} `type:"structure" payload:"Body"`

	// The bytes to be stored.
	//
	// Body is a required field
	Body io.ReadSeeker `type:"blob" required:"true"`

	// An optional CacheControl header that allows the caller to control the object's
	// cache behavior. Headers can be passed in as specified in the HTTP at https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9
	// (https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9).
	//
	// Headers with a custom user-defined value are also accepted.
	CacheControl *string `location:"header" locationName:"Cache-Control" type:"string"`

	// The content type of the object.
	ContentType *string `location:"header" locationName:"Content-Type" type:"string"`

	// The path (including the file name) where the object is stored in the container.
	// Format: <folder name>/<folder name>/<file name>
	//
	// For example, to upload the file mlaw.avi to the folder path premium\canada
	// in the container movies, enter the path premium/canada/mlaw.avi.
	//
	// Do not include the container name in this path.
	//
	// If the path includes any folders that don't exist yet, the service creates
	// them. For example, suppose you have an existing premium/usa subfolder. If
	// you specify premium/canada, the service creates a canada subfolder in the
	// premium folder. You then have two subfolders, usa and canada, in the premium
	// folder.
	//
	// There is no correlation between the path to the source and the path (folders)
	// in the container in AWS Elemental MediaStore.
	//
	// For more information about folders and how they exist in a container, see
	// the AWS Elemental MediaStore User Guide (http://docs.aws.amazon.com/mediastore/latest/ug/).
	//
	// The file name is the name that is assigned to the file that you upload. The
	// file can have the same name inside and outside of AWS Elemental MediaStore,
	// or it can have the same name. The file name can include or omit an extension.
	//
	// Path is a required field
	Path *string `location:"uri" locationName:"Path" min:"1" type:"string" required:"true"`

	// Indicates the storage class of a Put request. Defaults to high-performance
	// temporal storage class, and objects are persisted into durable storage shortly
	// after being received.
	StorageClass *string `location:"header" locationName:"x-amz-storage-class" min:"1" type:"string" enum:"StorageClass"`
}

// String returns the string representation
func (s PutObjectInput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s PutObjectInput) GoString() string {
	return s.String()
}

// Validate inspects the fields of the type to determine if they are valid.
func (s *PutObjectInput) Validate() error {
	invalidParams := request.ErrInvalidParams{Context: "PutObjectInput"}
	if s.Body == nil {
		invalidParams.Add(request.NewErrParamRequired("Body"))
	}
	if s.Path == nil {
		invalidParams.Add(request.NewErrParamRequired("Path"))
	}
	if s.Path != nil && len(*s.Path) < 1 {
		invalidParams.Add(request.NewErrParamMinLen("Path", 1))
	}
	if s.StorageClass != nil && len(*s.StorageClass) < 1 {
		invalidParams.Add(request.NewErrParamMinLen("StorageClass", 1))
	}

	if invalidParams.Len() > 0 {
		return invalidParams
	}
	return nil
}

// SetBody sets the Body field's value.
func (s *PutObjectInput) SetBody(v io.ReadSeeker) *PutObjectInput {
	s.Body = v
	return s
}

// SetCacheControl sets the CacheControl field's value.
func (s *PutObjectInput) SetCacheControl(v string) *PutObjectInput {
	s.CacheControl = &v
	return s
}

// SetContentType sets the ContentType field's value.
func (s *PutObjectInput) SetContentType(v string) *PutObjectInput {
	s.ContentType = &v
	return s
}

// SetPath sets the Path field's value.
func (s *PutObjectInput) SetPath(v string) *PutObjectInput {
	s.Path = &v
	return s
}

// SetStorageClass sets the StorageClass field's value.
func (s *PutObjectInput) SetStorageClass(v string) *PutObjectInput {
	s.StorageClass = &v
	return s
}

type PutObjectOutput struct {
	_ struct{} `type:"structure"`

	// The SHA256 digest of the object that is persisted.
	ContentSHA256 *string `min:"64" type:"string"`

	// Unique identifier of the object in the container.
	ETag *string `min:"1" type:"string"`

	// The storage class where the object was persisted. Should be “Temporal”.
	StorageClass *string `min:"1" type:"string" enum:"StorageClass"`
}

// String returns the string representation
func (s PutObjectOutput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s PutObjectOutput) GoString() string {
	return s.String()
}

// SetContentSHA256 sets the ContentSHA256 field's value.
func (s *PutObjectOutput) SetContentSHA256(v string) *PutObjectOutput {
	s.ContentSHA256 = &v
	return s
}

// SetETag sets the ETag field's value.
func (s *PutObjectOutput) SetETag(v string) *PutObjectOutput {
	s.ETag = &v
	return s
}

// SetStorageClass sets the StorageClass field's value.
func (s *PutObjectOutput) SetStorageClass(v string) *PutObjectOutput {
	s.StorageClass = &v
	return s
}

const (
	// ItemTypeObject is a ItemType enum value
	ItemTypeObject = "OBJECT"

	// ItemTypeFolder is a ItemType enum value
	ItemTypeFolder = "FOLDER"
)

const (
	// StorageClassTemporal is a StorageClass enum value
	StorageClassTemporal = "TEMPORAL"
)
