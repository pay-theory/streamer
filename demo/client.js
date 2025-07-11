const WebSocket = require('ws');
const readline = require('readline');

// Configuration - Updated for DynamORM Streamer
const WS_URL = process.env.WS_URL || 'wss://your-api.execute-api.region.amazonaws.com/prod';
const CONNECTION_ID = process.env.CONNECTION_ID || 'demo-conn-12345678';
const JWT_TOKEN = process.env.JWT_TOKEN || 'your-jwt-token-here';

// Create readline interface for user input
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout
});

// Create WebSocket connection with JWT authentication
console.log(`Connecting to ${WS_URL}...`);
const wsUrl = `${WS_URL}?Authorization=${JWT_TOKEN}`;
const ws = new WebSocket(wsUrl);

// Connection opened
ws.on('open', () => {
  console.log('✅ Connected to Streamer (DynamORM architecture)!');
  console.log(`📝 Using connection ID: ${CONNECTION_ID}`);
  console.log('\n🔥 Available commands (Updated for DynamORM):');
  console.log('  1. ping               - Fast sync ping (< 5s)');
  console.log('  2. echo <message>     - Test sync echo');
  console.log('  3. report             - Generate async report with progress');
  console.log('  4. echo_async <msg>   - Test async echo with progress');
  console.log('  5. knowledge <query>  - Query knowledge base async');
  console.log('  6. data               - Process data async');
  console.log('  7. exit               - Close connection\n');
  
  promptUser();
});

// Handle incoming messages
ws.on('message', (data) => {
  const msg = JSON.parse(data);
  
  if (msg.type === 'progress') {
    // Display progress bar
    const percentage = Math.floor(msg.percentage);
    const filled = Math.floor(percentage / 2);
    const bar = '█'.repeat(filled) + '░'.repeat(50 - filled);
    process.stdout.write(`\r[${bar}] ${percentage}% - ${msg.message}`);
    
    if (percentage === 100) {
      console.log('\n');
      promptUser();
    }
  } else if (msg.type === 'complete') {
    console.log('\n✅ Request completed!');
    console.log('Result:', JSON.stringify(msg.result, null, 2));
    promptUser();
  } else if (msg.type === 'error') {
    console.log('\n❌ Error:', msg.error.message);
    promptUser();
  } else {
    console.log('\n📨 Message:', JSON.stringify(msg, null, 2));
    promptUser();
  }
});

// Handle errors
ws.on('error', (error) => {
  console.error('❌ WebSocket error:', error);
});

// Handle connection close
ws.on('close', () => {
  console.log('\n👋 Connection closed');
  process.exit(0);
});

// Prompt for user input
function promptUser() {
  rl.question('\n> ', (input) => {
    const [command, ...args] = input.trim().split(' ');
    
    switch (command) {
      case 'ping':
        sendRequest('ping', {});
        break;
        
      case 'echo':
        sendRequest('echo', { message: args.join(' ') || 'Hello, World!' });
        break;
        
      case 'report':
        sendRequest('generate_report', {
          start_date: '2024-01-01',
          end_date: '2024-01-31',
          format: 'pdf',
          report_type: 'monthly'
        });
        break;
        
      case 'echo_async':
        sendRequest('echo_async', { 
          message: args.join(' ') || 'Testing async echo',
          timestamp: new Date().toISOString()
        });
        break;
        
      case 'knowledge':
        sendRequest('knowledge_query', {
          query: args.join(' ') || 'What is async processing?',
          knowledge_base_id: 'demo-kb-123',
          max_results: 5
        });
        break;
        
      case 'data':
        sendRequest('process_data', {
          dataset_id: 'demo-dataset-123',
          operation: 'transform'
        });
        break;
        
      case 'exit':
        console.log('Closing connection...');
        ws.close();
        rl.close();
        break;
        
      default:
        console.log('Unknown command:', command);
        promptUser();
    }
  });
}

// Send request to WebSocket using proper Streamer message format
function sendRequest(action, payload) {
  const request = {
    id: `req-${Date.now()}`,        // Client-generated request ID
    action: action,                 // Handler to invoke
    payload: payload,               // Handler-specific payload
    metadata: {                     // Optional request metadata
      client_version: "1.0.0",
      demo_client: true
    }
  };
  
  console.log('📤 Sending:', action);
  console.log('📋 Request ID:', request.id);
  ws.send(JSON.stringify(request));
} 