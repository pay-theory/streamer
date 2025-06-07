const WebSocket = require('ws');
const readline = require('readline');

// Configuration
const WS_URL = process.env.WS_URL || 'wss://your-api.execute-api.region.amazonaws.com/prod';
const CONNECTION_ID = process.env.CONNECTION_ID || 'demo-conn-12345678';

// Create readline interface for user input
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout
});

// Create WebSocket connection
console.log(`Connecting to ${WS_URL}...`);
const ws = new WebSocket(WS_URL);

// Connection opened
ws.on('open', () => {
  console.log('✅ Connected to WebSocket!');
  console.log(`📝 Using connection ID: ${CONNECTION_ID}`);
  console.log('\nAvailable commands:');
  console.log('  1. echo <message>     - Test sync echo');
  console.log('  2. report             - Generate async report');
  console.log('  3. echo_async <msg>   - Test async echo with progress');
  console.log('  4. data               - Process data async');
  console.log('  5. exit               - Close connection\n');
  
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

// Send request to WebSocket
function sendRequest(action, payload) {
  const request = {
    action: action,
    connection_id: CONNECTION_ID,
    request_id: `req-${Date.now()}`,
    payload: payload
  };
  
  console.log('📤 Sending:', action);
  ws.send(JSON.stringify(request));
} 