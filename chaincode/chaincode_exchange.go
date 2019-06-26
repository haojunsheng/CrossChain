package main

import (
"bytes"
"encoding/json"
"fmt"
"github.com/hyperledger/fabric/core/chaincode/shim"
sc "github.com/hyperledger/fabric/protos/peer"
	"strconv"
)

// Define the Smart Contract structure
type SmartContract struct {
}

// step equals 0 means failed，1 means 锁定chain1数据，并读取至chain3，
// 2 means 将数据做转换并log， 3 means 将数据写入chain2并锁定
// 4 means  将 chain1数据置为已转移 5 means 将chain2数据置为正常
//6 means finish
//
type Exchange struct {
	From   string `json:"from"`
	To  string `json:"to"`
	DateTime string `json:"datetime"`
	Step string `json:"step"`
}

type Car struct {
	Make   string `json:"make"`
	Model  string `json:"model"`
	Colour string `json:"colour"`
	Owner  string `json:"owner"`
	Key string `json:"key"`
	Trans string `json:"trans"`
	DateTime string `json:"datetime"`
	Status string `json:"status"`
}

/*
 * The Init method is called when the Smart Contract "fabcar" is instantiated by the blockchain network
 * Best practice is to have any Ledger initialization in separate function -- see initLedger()
 */
func (s *SmartContract) Init(APIstub shim.ChaincodeStubInterface) sc.Response {
	return shim.Success(nil)
}

/*
 * The Invoke method is called as a result of an application request to run the Smart Contract "fabcar"
 * The calling application program has also specified the particular smart contract function to be called, with arguments
 */
func (s *SmartContract) Invoke(APIstub shim.ChaincodeStubInterface) sc.Response {

	// Retrieve the requested Smart Contract function and arguments
	function, args := APIstub.GetFunctionAndParameters()
	// Route to the appropriate handler function to interact with the ledger appropriately
	if function == "queryCar" {
		return s.queryCar(APIstub, args)
	} else if function == "initLedger" {
		return s.initLedger(APIstub)
	} else if function == "createCar" {
		return s.createCar(APIstub, args)
	} else if function == "queryAllCars" {
		return s.queryAllCars(APIstub)
	} else if function == "exchange"{
		return s.exchange(APIstub,args)
	}

	return shim.Error("Invalid Smart Contract function name.")
}

func toChaincodeArgs(args ...string) [][]byte {
	bargs := make([][]byte, len(args))
	for i, arg := range args {
		bargs[i] = []byte(arg)
	}
	return bargs
}

//完成跨链工作
func (s *SmartContract) exchange(APIstub shim.ChaincodeStubInterface, args []string) sc.Response {

	if len(args) != 3 {
		return shim.Error("Incorrect number of arguments. Expecting 3")
	}

	key:=args[0];//example: key_code1_001
	chaincode1:=args[1];
	chaincode2:=args[2];

	exchange:=Exchange{ }
	exchange.From=chaincode1
	exchange.To=chaincode2

	//1. 锁定chain1数据并读取到chain3
	exchange.Step="1"
	invokeArgs := toChaincodeArgs("queryCar", key)
	chain1response := APIstub.InvokeChaincode(chaincode1, invokeArgs, "channel1")
	if chain1response.Status != shim.OK {
		exchange.Step="0"
		errStr := fmt.Sprintf("Cross chain error.Failed to invoke chaincode.")
		fmt.Printf(errStr)
		return shim.Error(errStr)
	}
	car := Car{}
	json.Unmarshal(chain1response.Payload, &car)
	if car.Status=="0"{
		errStr := fmt.Sprintf("Cross chain error: chaincode1 status 0 非正常")
		fmt.Printf(errStr)
		return shim.Error(errStr)
	}else if car.Status=="2"{
		errStr := fmt.Sprintf("Cross chain error: chaincode1 status 2 已锁定")
		fmt.Printf(errStr)
		return shim.Error(errStr)
	}else if car.Status=="3"{
		errStr := fmt.Sprintf("Cross chain error: chaincode1 status 3 已转移")
		fmt.Printf(errStr)
		return shim.Error(errStr)
	}
	car.Status="2"//修改状态值为锁定
	carModiStatus, _ := json.Marshal(car)
	invokeArgs = toChaincodeArgs("setStatus", key,string(carModiStatus))
	lockChaincode1 := APIstub.InvokeChaincode(chaincode1, invokeArgs, "channel1")
	if lockChaincode1.Status!=shim.OK{
		exchange.Step="0"
		errStr := fmt.Sprintf("Cross chain error: lock chaincode1 status error")
		fmt.Printf(errStr)
		return shim.Error(errStr)
	}
	//2. 数据进行转换并且锁定
	exchange.Step="2"

	//3.数据写入chain2并锁定
	exchange.Step="3"
	invokeArgs = toChaincodeArgs("createCar", key,string(carModiStatus))
	putChaincode2 := APIstub.InvokeChaincode(chaincode2, invokeArgs, "channel2")
	if putChaincode2.Status!=shim.OK{
		exchange.Step="0"
		errStr := fmt.Sprintf("Cross chain error: putData chaincode2 error. Rollback chaincode1 status")
		fmt.Printf(errStr)
		// TODO: ROLL BACK
		return shim.Error(errStr)
	}
	//4.Chain1设为已转移
	exchange.Step="4"
	car.Status="3"//修改状态值为已转移
	carModiStatus, _ = json.Marshal(car)
	invokeArgs = toChaincodeArgs("setStatus", key,string(carModiStatus))
	tranChaincode1 := APIstub.InvokeChaincode(chaincode1, invokeArgs, "channel1")
	if tranChaincode1.Status!=shim.OK{
		exchange.Step="0"
		//TODO : ROLLBACK
		errStr := fmt.Sprintf("Cross chain error: modified chaincode1 status transfered error")
		fmt.Printf(errStr)
		return shim.Error(errStr)
	}
	//5. chain2设置为正常
	exchange.Step="5"
	car.Status="1"//修改状态值为正常
	carModiStatus, _ = json.Marshal(car)
	invokeArgs = toChaincodeArgs("setStatus", key,string(carModiStatus))
	normalChaincode2:= APIstub.InvokeChaincode(chaincode2, invokeArgs, "channel2")
	if normalChaincode2.Status!=shim.OK{
		exchange.Step="0"
		//TODO : ROLLBACK
		errStr := fmt.Sprintf("Cross chain error: modified chaincode2 status normal error")
		fmt.Printf(errStr)
		return shim.Error(errStr)
	}
	exchange.Step="6"
	return shim.Success([]byte("cross chain success"))
}

func (s *SmartContract) queryCar(APIstub shim.ChaincodeStubInterface, args []string) sc.Response {

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	carAsBytes, _ := APIstub.GetState(args[0])
	return shim.Success(carAsBytes)
}

func (s *SmartContract) initLedger(APIstub shim.ChaincodeStubInterface) sc.Response {
	cars := []Car{
		Car{Make: "Toyota", Model: "Prius", Colour: "blue", Owner: "Tomoko",Key:"key_code1_001",DateTime:"20190621",Status:"0"},
		Car{Make: "Ford", Model: "Mustang", Colour: "red", Owner: "Brad",Key:"key_code1_002",DateTime:"20190621",Status:"0"},
		Car{Make: "Hyundai", Model: "Tucson", Colour: "green", Owner: "Jin Soo",Key:"key_code1_003",DateTime:"20190622",Status:"0"},
		Car{Make: "Volkswagen", Model: "Passat", Colour: "yellow", Owner: "Max",Key:"key_code1_004",DateTime:"20190623",Status:"0"},
		Car{Make: "Tesla", Model: "S", Colour: "black", Owner: "Adriana",Key:"key_code1_005",DateTime:"20190623",Status:"0"},
		Car{Make: "Peugeot", Model: "205", Colour: "purple", Owner: "Michel",Key:"key_code1_006",DateTime:"20190623",Status:"0"},
		Car{Make: "Chery", Model: "S22L", Colour: "white", Owner: "Aarav",Key:"key_code1_007",DateTime:"20190623",Status:"0"},
		Car{Make: "Fiat", Model: "Punto", Colour: "violet", Owner: "Pari",Key:"key_code1_008",DateTime:"20190624",Status:"0"},
		Car{Make: "Tata", Model: "Nano", Colour: "indigo", Owner: "Valeria",Key:"key_code1_009",DateTime:"20190624",Status:"0"},
		Car{Make: "Holden", Model: "Barina", Colour: "brown", Owner: "Shotaro",Key:"key_code1_010",DateTime:"20190624",Status:"0"},
	}

	i := 0
	for i < len(cars) {
		fmt.Println("i is ", i)
		carAsBytes, _ := json.Marshal(cars[i])
		APIstub.PutState("CAR"+strconv.Itoa(i), carAsBytes)
		fmt.Println("Added", cars[i])
		i = i + 1
	}

	return shim.Success(nil)
}

func (s *SmartContract) createCar(APIstub shim.ChaincodeStubInterface, args []string) sc.Response {

	if len(args) != 5 {
		return shim.Error("Incorrect number of arguments. Expecting 5")
	}

	var car = Car{Make: args[1], Model: args[2], Colour: args[3], Owner: args[4],Key:args[6],DateTime:args[7],Status:args[8]}

	carAsBytes, _ := json.Marshal(car)
	APIstub.PutState(args[0], carAsBytes)

	return shim.Success(nil)
}

func (s *SmartContract) queryAllCars(APIstub shim.ChaincodeStubInterface) sc.Response {

	startKey := "CAR0"
	endKey := "CAR999"

	resultsIterator, err := APIstub.GetStateByRange(startKey, endKey)
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing QueryResults
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}
		buffer.WriteString("{\"Key\":")
		buffer.WriteString("\"")
		buffer.WriteString(queryResponse.Key)
		buffer.WriteString("\"")

		buffer.WriteString(", \"Record\":")
		// Record is a JSON object, so we write as-is
		buffer.WriteString(string(queryResponse.Value))
		buffer.WriteString("}")
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")

	fmt.Printf("- queryAllCars:\n%s\n", buffer.String())

	return shim.Success(buffer.Bytes())
}

// The main function is only relevant in unit test mode. Only included here for completeness.
func main() {
	// Create a new Smart Contract
	err := shim.Start(new(SmartContract))
	if err != nil {
		fmt.Printf("Error creating new Smart Contract: %s", err)
	}
}
