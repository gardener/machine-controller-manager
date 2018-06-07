package infraserver

import (
	"errors"
	"fmt"
	"sync/atomic"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pb "github.com/gardener/machine-controller-manager/pkg/grpc/infrapb"
	"github.com/golang/glog"
)

// MachineClassMeta has metadata about the machine class.
type MachineClassMeta struct {
	Name     string
	Revision int32
}

// Driver interface mediates the communication with the external driver
type Driver interface {
	Create(machineClass *MachineClassMeta, credentials, machineID, machineName string) (string, string, error)
	Delete(credentials, machineID string) error
}

// driver also implements the interface Infragrpc_RegisterServer as a proxy to unregister the driver automatically on error during Send or Recv.
type driver struct {
	machineClassType metav1.TypeMeta
	stream           pb.Infragrpc_RegisterServer
	stopCh           chan interface{}
	requestCounter   int32
	pendingRequests  map[int32](chan *pb.DriverSide)
}

// send proxies to the stream but closes the driver on error.
func (d *driver) send(msg *pb.MCMside) error {
	err := d.stream.Send(msg)
	if err != nil {
		glog.Warning("Error sending message %v: %s. Closing the driver.", msg, err)
		d.close()
	}

	return err
}

// recv proxies to the stream but closes the driver on error.
func (d *driver) recv() (*pb.DriverSide, error) {
	msg, err := d.stream.Recv()
	if err != nil {
		glog.Warning("Error receiving message %v: %s. Closing the driver.", msg, err)
		d.close()
	}

	return msg, err
}

func (d *driver) close() {
	ch := d.stopCh
	if ch != nil {
		d.stopCh = nil
		close(ch)
	}
}

func (d *driver) wait() {
	if d.stopCh != nil {
		<-d.stopCh
	}
}

func (d *driver) nextRequestID() int32 {
	return atomic.AddInt32(&d.requestCounter, 1)
}

func (d *driver) receiveAndDispatch() error {
	for {
		msg, err := d.recv()
		if err != nil {
			return err
		}

		if msg.OperationType == "unregister" {
			d.close()
			return nil
		}

		if ch, ok := d.pendingRequests[msg.OperationID]; ok {
			ch <- msg
		} else {
			glog.Warningf("Request ID %d missing in pending requests", msg.OperationID)
		}
	}
}

func (d *driver) sendAndWait(params *pb.MCMsideOperationParams, opType string) (interface{}, error) {
	id := d.nextRequestID()
	msg := pb.MCMside{
		OperationID:     id,
		OperationType:   opType,
		Operationparams: params,
	}

	if err := d.send(&msg); err != nil {
		glog.Fatalf("Failed to send request: %v", err)
		return nil, err
	}

	ch := make(chan *pb.DriverSide)
	//TODO validation
	d.pendingRequests[id] = ch

	// The receiveDriverStream function will receive message, read the opID, then write to corresponding waitc
	// This will make sure that the response structure is populated
	response := <-ch

	delete(d.pendingRequests, id)
	close(ch)

	if response == nil {
		return nil, fmt.Errorf("Received nil response from driver %v", d.machineClassType)
	}

	return response.GetResponse(), nil
}

// Create sends create request to the driver over the grpc stream
func (d *driver) Create(machineClass *MachineClassMeta, credentials, machineID, machineName string) (string, string, error) {
	createParams := pb.MCMsideOperationParams{
		Credentials: credentials,
		MachineID:   machineID,
		MachineName: machineName,
	}
	if machineClass != nil {
		createParams.MachineClassMetaData = &pb.MachineClassMeta{
			Name:     machineClass.Name,
			Revision: machineClass.Revision,
		}
	}

	createResp, err := d.sendAndWait(&createParams, "create")
	if err != nil {
		glog.Fatalf("Failed to send create req: %v", err)
		return "", "", err
	}

	if createResp == nil {
		return "", "", fmt.Errorf("Create response empty")
	}

	//TODO type check
	response := createResp.(*pb.DriverSide_Createresponse).Createresponse
	glog.Infof("Create. Return: %s %s %s", response.ProviderID, response.Nodename, response.Error)

	err = nil
	if response.Error != "" {
		err = errors.New(response.Error)
	}

	return response.ProviderID, response.Nodename, err
}

// Delete sends delete request to the driver over the grpc stream
func (d *driver) Delete(credentials, machineID string) error {
	deleteParams := pb.MCMsideOperationParams{
		Credentials: credentials,
		MachineID:   machineID,
	}

	deleteResp, err := d.sendAndWait(&deleteParams, "delete")
	if err != nil {
		return err
	}

	if deleteResp == nil {
		return fmt.Errorf("Delete response empty")
	}
	response := deleteResp.(*pb.DriverSide_Deleteresponse).Deleteresponse
	glog.Infof("Delete Return: %s", response.Error)
	if response.Error != "" {
		return errors.New(response.Error)
	}
	return nil
}
