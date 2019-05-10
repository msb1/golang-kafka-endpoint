var p_socket;
var c_socket;
var epd_socket;

//
//$(document).ready(function(){
//    //connect to the socket server.
//    producer_socket = io.connect('http://' + document.domain + ':' + location.port + '/producer');
//    var msg_received = [];
//
//    //receive details from server
//    producer_socket.on('newMessage', function(msg) {
//        console.log("Received producer" + msg.num + ": " + msg.msg);
//        num = msg.num.toString();
//        //maintain a list of ten numbers
//        if (msg_received.length >= 4){
//            msg_received.shift()
//        }            
//        msg_received.push(msg.msg);
//        msg_string = '';
//        for (var i = 0; i < msg_received.length; i++){
//            msg_string = msg_string + '<p>' + msg_received[i].toString() + '</p>';
//        }
//        $('#producer' + num).html(msg_string);
//    });
//});
//
//$(document).ready(function(){
//    //connect to the socket server.
//    consumer_socket = io.connect('http://' + document.domain + ':' + location.port + '/consumer');
//    var msg_received = [];
//
//    //receive details from server
//    consumer_socket.on('newMessage', function(msg) {
//    	console.log("Received consumer" + msg.num + ": " + msg.msg);
//    	num = msg.num.toString();
//        //maintain a list of ten numbers
//        if (msg_received.length >= 4){
//            msg_received.shift()
//        }            
//        msg_received.push(msg.msg);
//        msg_string = '';
//        for (var i = 0; i < msg_received.length; i++){
//            msg_string = msg_string + '<p>' + msg_received[i].toString() + '</p>';
//        }
//        $('#consumer' + num).html(msg_string);
//    });
//});
//
//$(document).ready(function(){
//    //connect to the socket server.
//    epd_socket = io.connect('http://' + document.domain + ':' + location.port + '/epd');
//
//    //receive details from server
//    epd_socket.on('newMessage', function(msg) {
//        console.log("Received status" + msg.num + ": " + msg.msg);
//        num = msg.num.toString();
//        $('#status' + num).html(msg.msg.toString());
//    });
//});


$(document).ready(function(){
    //connect to the socket server.
    epd_socket = io.connect('http://' + document.domain + ':' + location.port + '/epd');
    p_socket = io.connect('http://' + document.domain + ':' + location.port + '/producer');
    c_socket = io.connect('http://' + document.domain + ':' + location.port + '/consumer');
});


function start_producer(num) {
	p_socket.emit('message', 'start,' + num);
}

function start_consumer(num) {
	c_socket.emit('message', '{start,' + num);
}

function stop_producer() {
	p_socket.emit('message', 'stop');
}

function stop_consumer() {
	c_socket.emit('message', 'stop');
}


