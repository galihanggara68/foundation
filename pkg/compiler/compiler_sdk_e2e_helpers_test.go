package compiler_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const e2eSharedBlueprint = `
actor CustomerActor {
  id: uuid
  role: string
  is_verified: bool
}

enum OrderStatus {
  pending
  confirmed
  cancelled
}

value PlaceOrderInput {
  product_id: uuid
  quantity: int
}

entity Product {
  id: uuid @pk
  name: string
  sku: string @unique
  price: int
  stock: int = 0
}

entity Order {
  id: uuid @pk
  customer_id: uuid
  product_id: uuid
  quantity: int
  total: int
  status: OrderStatus = .pending
}

error ProductNotFound {
  kind: not_found
  message: "product not found"
}

error InsufficientStock {
  kind: conflict
  message: "insufficient stock"
}

error UnauthorizedCustomer {
  kind: forbidden
  message: "customer is not authorized"
}

policy CanPlaceOrder(actor: Actor) {
  actor.role == "customer" && actor.is_verified == true
}

action CreateProduct(name: string, sku: string, price: int, stock: int) -> Product
  @http(method: POST, path: "/v1/products")
{
  let p = create Product {
    name: name,
    sku: sku,
    price: price,
    stock: stock
  }
  return p
}

action PlaceOrder(input: PlaceOrderInput) -> Order
  @http(method: POST, path: "/v1/orders")
{
  require CanPlaceOrder(actor) else UnauthorizedCustomer

  let p = query one Product where id == input.product_id else ProductNotFound
  guard p.stock >= input.quantity else InsufficientStock

  update Product where id == p.id set {
    stock: p.stock - input.quantity
  }

  let total = p.price * input.quantity
  let o = create Order {
    customer_id: actor.id,
    product_id: p.id,
    quantity: input.quantity,
    total: total,
    status: .confirmed
  }
  return o
}

action ListOrders(cust_id: uuid, cursor: string) -> []Order
  @http(method: GET, path: "/v1/orders")
{
  let orders = query many Order
    where customer_id == cust_id
    order_by id asc
    limit 2
    cursor cursor
  return orders
}
`

func makeTestJWT(sub, role string, verified bool) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims := map[string]interface{}{
		"sub":         sub,
		"role":        role,
		"is_verified": verified,
	}
	payloadBytes, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return fmt.Sprintf("%s.%s.sig", header, payload)
}

const (
	testValidActorID      = "00000000-0000-0000-0000-000000000001"
	testUnverifiedActorID = "00000000-0000-0000-0000-000000000002"
)

func getTestTokens() (string, string) {
	return makeTestJWT(testValidActorID, "customer", true),
		makeTestJWT(testUnverifiedActorID, "customer", false)
}

func getFreePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForPort(t *testing.T, port int) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server on %s failed to become ready within deadline", addr)
}

func buildAndStartGoServer(t *testing.T, targetDir string) (string, func()) {
	binPath := filepath.Join(targetDir, "server-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	buildCmd.Dir = targetDir
	buildCmd.Env = append(os.Environ(), "GOPROXY=off")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build in %s failed: %v\n%s", targetDir, err, string(out))
	}

	port := getFreePort(t)
	srvCmd := exec.Command(binPath)
	srvCmd.Dir = targetDir
	srvCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	if err := srvCmd.Start(); err != nil {
		t.Fatalf("failed to start Go server binary: %v", err)
	}

	cleanup := func() {
		if srvCmd.Process != nil {
			_ = srvCmd.Process.Signal(syscall.SIGTERM)
			_ = srvCmd.Wait()
		}
	}

	waitForPort(t, port)
	return fmt.Sprintf("http://127.0.0.1:%d", port), cleanup
}

func startTSServer(t *testing.T, targetDir string) (string, func()) {
	// Install express/pg dependencies
	npmCmd := exec.Command("npm", "install", "--prefer-offline", "--no-audit", "--no-fund", "--silent")
	npmCmd.Dir = targetDir
	if out, err := npmCmd.CombinedOutput(); err != nil {
		t.Fatalf("npm install in %s failed: %v\n%s", targetDir, err, string(out))
	}

	port := getFreePort(t)
	srvCmd := exec.Command("node", "--experimental-strip-types", "src/main/server.ts")
	srvCmd.Dir = targetDir
	srvCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	if err := srvCmd.Start(); err != nil {
		t.Fatalf("failed to start TypeScript server: %v", err)
	}

	cleanup := func() {
		if srvCmd.Process != nil {
			_ = srvCmd.Process.Signal(syscall.SIGTERM)
			_ = srvCmd.Wait()
		}
	}

	waitForPort(t, port)
	return fmt.Sprintf("http://127.0.0.1:%d", port), cleanup
}
