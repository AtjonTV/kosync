// Define JMP Message types
interface JMPMessage {
  jmp: string;
  proto: string;
  content: string;
  payload: any;
}

// Define callback types
type JmpRpcCallback = (data: any, typeHint: string, errors?: string[]) => void;
type JmpPubSubCallback = (data: any, typeHint: string, errors?: string[]) => void;
type JmpOnConnectedCallback = () => void;

// Client class
class JMPClient {
  private socket: WebSocket | null = null;
  private readonly rpcCallbacks: Map<string, JmpRpcCallback> = new Map();
  private readonly pubSubCallbacks: Map<string, JmpPubSubCallback> = new Map();

  private static readonly JmpVersion = "1";
  private static readonly JmpProtoRpc = "rpc";
  private static readonly JmpProtoPubSub = "pubsub";

  constructor(private readonly websocketUrl: string, private readonly debugLog?: boolean) {}

  // Connect to WebSocket
  public connect(onConnected?: JmpOnConnectedCallback): void {
    this.socket = new WebSocket(this.websocketUrl);
    this.socket.onopen = () => {
      if (this.debugLog)
        console.log('Connected to JMP server');
      if (onConnected) {
        onConnected();
      }
    };
    this.socket.onmessage = (event) => {
      this.handleMessage(event.data);
    };
    this.socket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
    this.socket.onclose = () => {
      if (this.debugLog)
        console.log('Disconnected from JMP server');
    };
  }

  // Handle incoming messages
  private handleMessage(data: string): void {
    if (this.debugLog)
      console.log("JMP Received Message: ", data)

    const message: JMPMessage = JSON.parse(data);

    if (message.proto === JMPClient.JmpProtoRpc) {
      const callback = this.rpcCallbacks.get(message.payload.method);
      if (callback) {
        callback(message.payload.data, message.payload.type_hint, message.payload.errors);
      }
    } else if (message.proto === JMPClient.JmpProtoPubSub) {
      const callback = this.pubSubCallbacks.get(message.payload.for_topic);
      if (callback) {
        callback(message.payload.data, message.payload.type_hint, message.payload.errors);
      }
    }
  }

  // Register RPC callback
  public registerRPCCallback(method: string, callback: JmpRpcCallback): void {
    if (this.debugLog)
      console.log(`JMP callback for RPC ${method} registered`)
    this.rpcCallbacks.set(method, callback);
  }

  // Register PubSub callback
  public registerPubSubCallback(topic: string, callback: JmpPubSubCallback): void {
    if (this.debugLog)
      console.log(`JMP callback for PubSub Topic ${topic} registered`)
    this.pubSubCallbacks.set(topic, callback);
  }

  // Send RPC request
  public rpc(type: string, data: any): void {
    if (this.socket?.readyState !== WebSocket.OPEN) {
      console.error("JMP Socket is not yet ready to send messages!");
      return;
    }
    const message: JMPMessage = {
      jmp: JMPClient.JmpVersion,
      proto: JMPClient.JmpProtoRpc,
      content: 'rpc.call',
      payload: {
        method: type,
        arguments: data
      }
    };
    if (this.debugLog)
      console.log("JMP RPC: ", JSON.stringify(message))
    this.socket.send(JSON.stringify(message));
  }

  public subscribe(topic: string): void {
    if (this.socket?.readyState !== WebSocket.OPEN) {
      console.error("JMP Socket is not yet ready to send messages!");
      return;
    }
    const message: JMPMessage = {
      jmp: JMPClient.JmpVersion,
      proto: JMPClient.JmpProtoPubSub,
      content: 'pubsub.subscribe',
      payload: {
        topic: topic
      }
    };
    if (this.debugLog)
      console.log("JMP Subscribe: ", JSON.stringify(message))
    this.socket.send(JSON.stringify(message));
  }

  // Close connection
  public close(): void {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.close();
    }
  }
}

// Export the client
export default JMPClient;
