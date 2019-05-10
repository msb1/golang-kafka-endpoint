### golang-kafka-endpoint

<h4> General Info </h4>
<ol>
  <li> 
    The purpose of this application is enable a Kafka endpoint simulator for testing Kafka, Kafka Connect and Kafka Streams with simulated analytics data streams
  </li>
  <li>
    In this test application, confluent-kafka-go is used to spawn a producer and consumer which are each enabled in a separate goroutines (actually one for each consumer and producer) 
    <ul>
      <li> Currently the producer and consumer defaults to bootstrap_servers='localhost:9092. Enter info in start menu for bootstrap-server and topics, etc.. </li>
      <li> There is an epd.conf file that should be edited to make changes in the simulator outputs. 
    </ul>
  </li>
  <li>
    Gorilla/mux router is used to replace the http golang default; the gorilla/websockets is used for duplex communication from the golang program to the front end javascript
  </li>
  <li> 
    The web page is based on the bootstrap sample dashboard from https://startbootstrap.com/previews/sb-admin-2/ so there is considerable flexibility in its application. There are currently a start, stop and close button in the left-side collapsable menu.
  </li>
  <li> 
    Multiple instances can be run simulataneously by connecting to the server (e.g., multiple web pages from the same browser). Concurrency is emphasized through the application of goroutines, channels and mutex locks 
  </li>
</ol>

