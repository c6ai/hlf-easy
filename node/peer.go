package node

import (
	"encoding/json"
	"fmt"
	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/process"
	log "github.com/sirupsen/logrus"
	"hlf-easy/certs"
	"hlf-easy/config"
	"hlf-easy/utils"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

type PeerNode struct {
	id        string
	cmdGetter func() (*exec.Cmd, error)
	cmd       *exec.Cmd
	p         *process.Process
	mspID     string
}
type PeerConfig struct {
	TLSCert  string `json:"tlsCert"`
	SignCert string `json:"signCert"`

	TLSCACert  string `json:"tlsCACert"`
	SignCACert string `json:"signCACert"`
}

func (n *PeerNode) GetConfig() (*PeerConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	peerDir := filepath.Join(home, fmt.Sprintf("hlf-easy/peers/%s", n.id))
	tlsCertBytes, err := os.ReadFile(filepath.Join(peerDir, "tls.crt"))
	if err != nil {
		return nil, err
	}
	signCertBytes, err := os.ReadFile(filepath.Join(peerDir, "signcerts/cert.pem"))
	if err != nil {
		return nil, err
	}
	signCACertBytes, err := os.ReadFile(filepath.Join(peerDir, "cacerts/cacert.pem"))
	if err != nil {
		return nil, err
	}
	tlsCACertBytes, err := os.ReadFile(filepath.Join(peerDir, "tlscacerts/cacert.pem"))
	if err != nil {
		return nil, err
	}
	return &PeerConfig{
		TLSCert:  string(tlsCertBytes),
		SignCert: string(signCertBytes),

		TLSCACert:  string(tlsCACertBytes),
		SignCACert: string(signCACertBytes),
	}, nil
}

func (n *PeerNode) GetID() string {
	return n.id
}

func (n *PeerNode) GetMSPID() string {
	return n.mspID
}

func (n *PeerNode) Start() error {
	if n.cmd != nil {
		log.Info("Peer node is already started")
		return errors.New("peer node is already started")
	}
	cmd, err := n.cmdGetter()
	if err != nil {
		log.Warnf("Failed to get peer node command: %v", err)
		return err
	}
	n.cmd = cmd
	if err := n.cmd.Start(); err != nil {
		log.Warnf("Failed to start peer node: %v", err)
		return err
	}

	p, err := process.NewProcess(int32(n.cmd.Process.Pid))
	if err != nil {
		log.Warnf("Failed to get peer node process: %v", err)
		return err
	}
	n.p = p
	return nil
}
func (n *PeerNode) Stop() error {
	if n.cmd == nil || n.cmd.Process == nil {
		log.Info("Peer node is already stopped")
		return errors.New("peer node is already stopped")
	}
	err := n.cmd.Process.Signal(os.Interrupt)
	if err != nil {
		log.Warnf("Failed to stop peer node: %v", err)
		return err
	}
	state, err := n.cmd.Process.Wait()
	if err != nil {
		log.Warnf("Failed to stop peer node: %v", err)
		return err
	}
	_ = state
	n.cmd = nil
	return nil
}

var StatusMap = map[string]string{
	"R": "Running",
	"S": "Sleep",
	"T": "Stop",
	"I": "Idle",
	"Z": "Zombie",
	"W": "Wait",
	"L": "Lock",
}

type ProcessState struct {
	PID        int                     `json:"pid"`
	Status     string                  `json:"status"`
	MemoryInfo *process.MemoryInfoStat `json:"memory"`
	CPUInfo    CPUInfo                 `json:"cpu"`
}
type CPUInfo struct {
	CPUPercent float64 `json:"percent"`
}

func (n *PeerNode) Status() (*ProcessState, error) {
	if n.cmd == nil || n.cmd.Process == nil {
		return &ProcessState{
			PID:    0,
			Status: "Stop",
			MemoryInfo: &process.MemoryInfoStat{
				RSS:    0,
				VMS:    0,
				HWM:    0,
				Data:   0,
				Stack:  0,
				Locked: 0,
				Swap:   0,
			},
			CPUInfo: CPUInfo{
				CPUPercent: 0,
			},
		}, nil
	}

	status, err := n.p.Status()
	if err != nil {
		log.Warnf("Failed to get peer node status: %v", err)
		return nil, err
	}
	statusStr, ok := StatusMap[status]
	if !ok {
		statusStr = "Unknown"
	}
	memoryInfo, err := n.p.MemoryInfo()
	if err != nil {
		log.Warnf("Failed to get peer node memory info: %v", err)
		return nil, err
	}
	cpuPercent, err := n.p.CPUPercent()
	if err != nil {
		log.Warnf("Failed to get peer node cpu percent: %v", err)
		return nil, err
	}
	ps := &ProcessState{
		PID:        int(n.p.Pid),
		Status:     statusStr,
		MemoryInfo: memoryInfo,
		CPUInfo: CPUInfo{
			CPUPercent: cpuPercent,
		},
	}
	return ps, nil
}

func NewPeerNode(
	id string,
	mspID string,
	cmdGetter func() (*exec.Cmd, error),
) *PeerNode {
	return &PeerNode{
		id:        id,
		mspID:     mspID,
		cmdGetter: cmdGetter,
	}
}

const (
	coreYamlTemplate = `
# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

###############################################################################
#
#    Peer section
#
###############################################################################
peer:

  # The peer id provides a name for this peer instance and is used when
  # naming docker resources.
  id: jdoe

  # The networkId allows for logical separation of networks and is used when
  # naming docker resources.
  networkId: dev

  # The Address at local network interface this Peer will listen on.
  # By default, it will listen on all network interfaces
  listenAddress: 0.0.0.0:7051

  # The endpoint this peer uses to listen for inbound chaincode connections.
  # If this is commented-out, the listen address is selected to be
  # the peer's address (see below) with port 7052
  # chaincodeListenAddress: 0.0.0.0:7052

  # The endpoint the chaincode for this peer uses to connect to the peer.
  # If this is not specified, the chaincodeListenAddress address is selected.
  # And if chaincodeListenAddress is not specified, address is selected from
  # peer address (see below). If specified peer address is invalid then it
  # will fallback to the auto detected IP (local IP) regardless of the peer
  # addressAutoDetect value.
  # chaincodeAddress: 0.0.0.0:7052

  # When used as peer config, this represents the endpoint to other peers
  # in the same organization. For peers in other organization, see
  # gossip.externalEndpoint for more info.
  # When used as CLI config, this means the peer's endpoint to interact with
  address: 0.0.0.0:7051

  # Whether the Peer should programmatically determine its address
  # This case is useful for docker containers.
  # When set to true, will override peer address.
  addressAutoDetect: false

  # Keepalive settings for peer server and clients
  keepalive:
    # Interval is the duration after which if the server does not see
    # any activity from the client it pings the client to see if it's alive
    interval: 7200s
    # Timeout is the duration the server waits for a response
    # from the client after sending a ping before closing the connection
    timeout: 20s
    # MinInterval is the minimum permitted time between client pings.
    # If clients send pings more frequently, the peer server will
    # disconnect them
    minInterval: 60s
    # Client keepalive settings for communicating with other peer nodes
    client:
      # Interval is the time between pings to peer nodes.  This must
      # greater than or equal to the minInterval specified by peer
      # nodes
      interval: 60s
      # Timeout is the duration the client waits for a response from
      # peer nodes before closing the connection
      timeout: 20s
    # DeliveryClient keepalive settings for communication with ordering
    # nodes.
    deliveryClient:
      # Interval is the time between pings to ordering nodes.  This must
      # greater than or equal to the minInterval specified by ordering
      # nodes.
      interval: 60s
      # Timeout is the duration the client waits for a response from
      # ordering nodes before closing the connection
      timeout: 20s


  # Gossip related configuration
  gossip:
    # Bootstrap set to initialize gossip with.
    # This is a list of other peers that this peer reaches out to at startup.
    # Important: The endpoints here have to be endpoints of peers in the same
    # organization, because the peer would refuse connecting to these endpoints
    # unless they are in the same organization as the peer.
    bootstrap: 127.0.0.1:7051

    # NOTE: orgLeader and useLeaderElection parameters are mutual exclusive.
    # Setting both to true would result in the termination of the peer
    # since this is undefined state. If the peers are configured with
    # useLeaderElection=false, make sure there is at least 1 peer in the
    # organization that its orgLeader is set to true.

    # Defines whenever peer will initialize dynamic algorithm for
    # "leader" selection, where leader is the peer to establish
    # connection with ordering service and use delivery protocol
    # to pull ledger blocks from ordering service.
    useLeaderElection: false
    # Statically defines peer to be an organization "leader",
    # where this means that current peer will maintain connection
    # with ordering service and disseminate block across peers in
    # its own organization. Multiple peers or all peers in an organization
    # may be configured as org leaders, so that they all pull
    # blocks directly from ordering service.
    orgLeader: true

    # Interval for membershipTracker polling
    membershipTrackerInterval: 5s

    # Overrides the endpoint that the peer publishes to peers
    # in its organization. For peers in foreign organizations
    # see 'externalEndpoint'
    endpoint:
    # Maximum count of blocks stored in memory
    maxBlockCountToStore: 10
    # Max time between consecutive message pushes(unit: millisecond)
    maxPropagationBurstLatency: 10ms
    # Max number of messages stored until a push is triggered to remote peers
    maxPropagationBurstSize: 10
    # Number of times a message is pushed to remote peers
    propagateIterations: 1
    # Number of peers selected to push messages to
    propagatePeerNum: 3
    # Determines frequency of pull phases(unit: second)
    # Must be greater than digestWaitTime + responseWaitTime
    pullInterval: 4s
    # Number of peers to pull from
    pullPeerNum: 3
    # Determines frequency of pulling state info messages from peers(unit: second)
    requestStateInfoInterval: 4s
    # Determines frequency of pushing state info messages to peers(unit: second)
    publishStateInfoInterval: 4s
    # Maximum time a stateInfo message is kept until expired
    stateInfoRetentionInterval:
    # Time from startup certificates are included in Alive messages(unit: second)
    publishCertPeriod: 10s
    # Should we skip verifying block messages or not (currently not in use)
    skipBlockVerification: false
    # Dial timeout(unit: second)
    dialTimeout: 3s
    # Connection timeout(unit: second)
    connTimeout: 2s
    # Buffer size of received messages
    recvBuffSize: 20
    # Buffer size of sending messages
    sendBuffSize: 200
    # Time to wait before pull engine processes incoming digests (unit: second)
    # Should be slightly smaller than requestWaitTime
    digestWaitTime: 1s
    # Time to wait before pull engine removes incoming nonce (unit: milliseconds)
    # Should be slightly bigger than digestWaitTime
    requestWaitTime: 1500ms
    # Time to wait before pull engine ends pull (unit: second)
    responseWaitTime: 2s
    # Alive check interval(unit: second)
    aliveTimeInterval: 5s
    # Alive expiration timeout(unit: second)
    aliveExpirationTimeout: 25s
    # Reconnect interval(unit: second)
    reconnectInterval: 25s
    # Max number of attempts to connect to a peer
    maxConnectionAttempts: 120
    # Message expiration factor for alive messages
    msgExpirationFactor: 20
    # This is an endpoint that is published to peers outside of the organization.
    # If this isn't set, the peer will not be known to other organizations.
    externalEndpoint:
    # Leader election service configuration
    election:
      # Longest time peer waits for stable membership during leader election startup (unit: second)
      startupGracePeriod: 15s
      # Interval gossip membership samples to check its stability (unit: second)
      membershipSampleInterval: 1s
      # Time passes since last declaration message before peer decides to perform leader election (unit: second)
      leaderAliveThreshold: 10s
      # Time between peer sends propose message and declares itself as a leader (sends declaration message) (unit: second)
      leaderElectionDuration: 5s

    pvtData:
      # pullRetryThreshold determines the maximum duration of time private data corresponding for a given block
      # would be attempted to be pulled from peers until the block would be committed without the private data
      pullRetryThreshold: 60s
      # As private data enters the transient store, it is associated with the peer's ledger's height at that time.
      # transientstoreMaxBlockRetention defines the maximum difference between the current ledger's height upon commit,
      # and the private data residing inside the transient store that is guaranteed not to be purged.
      # Private data is purged from the transient store when blocks with sequences that are multiples
      # of transientstoreMaxBlockRetention are committed.
      transientstoreMaxBlockRetention: 1000
      # pushAckTimeout is the maximum time to wait for an acknowledgement from each peer
      # at private data push at endorsement time.
      pushAckTimeout: 3s
      # Block to live pulling margin, used as a buffer
      # to prevent peer from trying to pull private data
      # from peers that is soon to be purged in next N blocks.
      # This helps a newly joined peer catch up to current
      # blockchain height quicker.
      btlPullMargin: 10
      # the process of reconciliation is done in an endless loop, while in each iteration reconciler tries to
      # pull from the other peers the most recent missing blocks with a maximum batch size limitation.
      # reconcileBatchSize determines the maximum batch size of missing private data that will be reconciled in a
      # single iteration.
      reconcileBatchSize: 10
      # reconcileSleepInterval determines the time reconciler sleeps from end of an iteration until the beginning
      # of the next reconciliation iteration.
      reconcileSleepInterval: 1m
      # reconciliationEnabled is a flag that indicates whether private data reconciliation is enable or not.
      reconciliationEnabled: true
      # skipPullingInvalidTransactionsDuringCommit is a flag that indicates whether pulling of invalid
      # transaction's private data from other peers need to be skipped during the commit time and pulled
      # only through reconciler.
      skipPullingInvalidTransactionsDuringCommit: false
      # implicitCollectionDisseminationPolicy specifies the dissemination  policy for the peer's own implicit collection.
      # When a peer endorses a proposal that writes to its own implicit collection, below values override the default values
      # for disseminating private data.
      # Note that it is applicable to all channels the peer has joined. The implication is that requiredPeerCount has to
      # be smaller than the number of peers in a channel that has the lowest numbers of peers from the organization.
      implicitCollectionDisseminationPolicy:
        # requiredPeerCount defines the minimum number of eligible peers to which the peer must successfully
        # disseminate private data for its own implicit collection during endorsement. Default value is 0.
        requiredPeerCount: 0
        # maxPeerCount defines the maximum number of eligible peers to which the peer will attempt to
        # disseminate private data for its own implicit collection during endorsement. Default value is 1.
        maxPeerCount: 1

    # Gossip state transfer related configuration
    state:
      # indicates whenever state transfer is enabled or not
      # default value is true, i.e. state transfer is active
      # and takes care to sync up missing blocks allowing
      # lagging peer to catch up to speed with rest network
      enabled: false
      # checkInterval interval to check whether peer is lagging behind enough to
      # request blocks via state transfer from another peer.
      checkInterval: 10s
      # responseTimeout amount of time to wait for state transfer response from
      # other peers
      responseTimeout: 3s
      # batchSize the number of blocks to request via state transfer from another peer
      batchSize: 10
      # blockBufferSize reflects the size of the re-ordering buffer
      # which captures blocks and takes care to deliver them in order
      # down to the ledger layer. The actual buffer size is bounded between
      # 0 and 2*blockBufferSize, each channel maintains its own buffer
      blockBufferSize: 20
      # maxRetries maximum number of re-tries to ask
      # for single state transfer request
      maxRetries: 3

  # TLS Settings
  tls:
    # Require server-side TLS
    enabled:  false
    # Require client certificates / mutual TLS.
    # Note that clients that are not configured to use a certificate will
    # fail to connect to the peer.
    clientAuthRequired: false
    # X.509 certificate used for TLS server
    cert:
      file: tls/server.crt
    # Private key used for TLS server (and client if clientAuthEnabled
    # is set to true
    key:
      file: tls/server.key
    # Trusted root certificate chain for tls.cert
    rootcert:
      file: tls/ca.crt
    # Set of root certificate authorities used to verify client certificates
    clientRootCAs:
      files:
        - tls/ca.crt
    # Private key used for TLS when making client connections.  If
    # not set, peer.tls.key.file will be used instead
    clientKey:
      file:
    # X.509 certificate used for TLS when making client connections.
    # If not set, peer.tls.cert.file will be used instead
    clientCert:
      file:

  # Authentication contains configuration parameters related to authenticating
  # client messages
  authentication:
    # the acceptable difference between the current server time and the
    # client's time as specified in a client request message
    timewindow: 15m

  # Path on the file system where peer will store data (eg ledger). This
  # location must be access control protected to prevent unintended
  # modification that might corrupt the peer operations.
  fileSystemPath: {{ .FileSystemPath }}

  # BCCSP (Blockchain crypto provider): Select which crypto implementation or
  # library to use
  BCCSP:
    Default: SW
    # Settings for the SW crypto provider (i.e. when DEFAULT: SW)
    SW:
      # TODO: The default Hash and Security level needs refactoring to be
      # fully configurable. Changing these defaults requires coordination
      # SHA2 is hardcoded in several places, not only BCCSP
      Hash: SHA2
      Security: 256
      # Location of Key Store
      FileKeyStore:
        # If "", defaults to 'mspConfigPath'/keystore
        KeyStore:
    # Settings for the PKCS#11 crypto provider (i.e. when DEFAULT: PKCS11)
    PKCS11:
      # Location of the PKCS11 module library
      Library:
      # Token Label
      Label:
      # User PIN
      Pin:
      Hash:
      Security:

  # Path on the file system where peer will find MSP local configurations
  mspConfigPath: msp

  # Identifier of the local MSP
  # ----!!!!IMPORTANT!!!-!!!IMPORTANT!!!-!!!IMPORTANT!!!!----
  # Deployers need to change the value of the localMspId string.
  # In particular, the name of the local MSP ID of a peer needs
  # to match the name of one of the MSPs in each of the channel
  # that this peer is a member of. Otherwise this peer's messages
  # will not be identified as valid by other nodes.
  localMspId: SampleOrg

  # CLI common client config options
  client:
    # connection timeout
    connTimeout: 3s

  # Delivery service related config
  deliveryclient:
    # It sets the total time the delivery service may spend in reconnection
    # attempts until its retry logic gives up and returns an error
    reconnectTotalTimeThreshold: 3600s

    # It sets the delivery service <-> ordering service node connection timeout
    connTimeout: 3s

    # It sets the delivery service maximal delay between consecutive retries
    reConnectBackoffThreshold: 3600s

    # A list of orderer endpoint addresses which should be overridden
    # when found in channel configurations.
    addressOverrides:
    #  - from:
    #    to:
    #    caCertsFile:
    #  - from:
    #    to:
    #    caCertsFile:

  # Type for the local MSP - by default it's of type bccsp
  localMspType: bccsp

  # Used with Go profiling tools only in none production environment. In
  # production, it should be disabled (eg enabled: false)
  profile:
    enabled:     false
    listenAddress: 0.0.0.0:6060

  # Handlers defines custom handlers that can filter and mutate
  # objects passing within the peer, such as:
  #   Auth filter - reject or forward proposals from clients
  #   Decorators  - append or mutate the chaincode input passed to the chaincode
  #   Endorsers   - Custom signing over proposal response payload and its mutation
  # Valid handler definition contains:
  #   - A name which is a factory method name defined in
  #     core/handlers/library/library.go for statically compiled handlers
  #   - library path to shared object binary for pluggable filters
  # Auth filters and decorators are chained and executed in the order that
  # they are defined. For example:
  # authFilters:
  #   -
  #     name: FilterOne
  #     library: /opt/lib/filter.so
  #   -
  #     name: FilterTwo
  # decorators:
  #   -
  #     name: DecoratorOne
  #   -
  #     name: DecoratorTwo
  #     library: /opt/lib/decorator.so
  # Endorsers are configured as a map that its keys are the endorsement system chaincodes that are being overridden.
  # Below is an example that overrides the default ESCC and uses an endorsement plugin that has the same functionality
  # as the default ESCC.
  # If the 'library' property is missing, the name is used as the constructor method in the builtin library similar
  # to auth filters and decorators.
  # endorsers:
  #   escc:
  #     name: DefaultESCC
  #     library: /etc/hyperledger/fabric/plugin/escc.so
  handlers:
    authFilters:
      -
        name: DefaultAuth
      -
        name: ExpirationCheck    # This filter checks identity x509 certificate expiration
    decorators:
      -
        name: DefaultDecorator
    endorsers:
      escc:
        name: DefaultEndorsement
        library:
    validators:
      vscc:
        name: DefaultValidation
        library:

  #    library: /etc/hyperledger/fabric/plugin/escc.so
  # Number of goroutines that will execute transaction validation in parallel.
  # By default, the peer chooses the number of CPUs on the machine. Set this
  # variable to override that choice.
  # NOTE: overriding this value might negatively influence the performance of
  # the peer so please change this value only if you know what you're doing
  validatorPoolSize:

  # The discovery service is used by clients to query information about peers,
  # such as - which peers have joined a certain channel, what is the latest
  # channel config, and most importantly - given a chaincode and a channel,
  # what possible sets of peers satisfy the endorsement policy.
  discovery:
    enabled: true
    # Whether the authentication cache is enabled or not.
    authCacheEnabled: true
    # The maximum size of the cache, after which a purge takes place
    authCacheMaxSize: 1000
    # The proportion (0 to 1) of entries that remain in the cache after the cache is purged due to overpopulation
    authCachePurgeRetentionRatio: 0.75
    # Whether to allow non-admins to perform non channel scoped queries.
    # When this is false, it means that only peer admins can perform non channel scoped queries.
    orgMembersAllowedAccess: false

  # Limits is used to configure some internal resource limits.
  limits:
    # Concurrency limits the number of concurrently running requests to a service on each peer.
    # Currently this option is only applied to endorser service and deliver service.
    # When the property is missing or the value is 0, the concurrency limit is disabled for the service.
    concurrency:
      # endorserService limits concurrent requests to endorser service that handles chaincode deployment, query and invocation,
      # including both user chaincodes and system chaincodes.
      endorserService: 2500
      # deliverService limits concurrent event listeners registered to deliver service for blocks and transaction events.
      deliverService: 2500

###############################################################################
#
#    VM section
#
###############################################################################
vm:

  # Endpoint of the vm management system.  For docker can be one of the following in general
  # unix:///var/run/docker.sock
  # http://localhost:2375
  # https://localhost:2376
  endpoint: ""

  # settings for docker vms
  docker:
    tls:
      enabled: false
      ca:
        file: docker/ca.crt
      cert:
        file: docker/tls.crt
      key:
        file: docker/tls.key

    # Enables/disables the standard out/err from chaincode containers for
    # debugging purposes
    attachStdout: false

    # Parameters on creating docker container.
    # Container may be efficiently created using ipam & dns-server for cluster
    # NetworkMode - sets the networking mode for the container. Supported
    # Dns - a list of DNS servers for the container to use.
    # Docker Host Config are not supported and will not be used if set.
    # LogConfig - sets the logging driver (Type) and related options
    # (Config) for Docker. For more info,
    # https://docs.docker.com/engine/admin/logging/overview/
    # Note: Set LogConfig using Environment Variables is not supported.
    hostConfig:
      NetworkMode: host
      Dns:
      # - 192.168.0.1
      LogConfig:
        Type: json-file
        Config:
          max-size: "50m"
          max-file: "5"
      Memory: 2147483648

###############################################################################
#
#    Chaincode section
#
###############################################################################
chaincode:

  # The id is used by the Chaincode stub to register the executing Chaincode
  # ID with the Peer and is generally supplied through ENV variables
  id:
    path:
    name:

  # Generic builder environment, suitable for most chaincode types
  builder: $(DOCKER_NS)/fabric-ccenv:$(TWO_DIGIT_VERSION)

  pull: false

  golang:
    # golang will never need more than baseos
    runtime: $(DOCKER_NS)/fabric-baseos:$(TWO_DIGIT_VERSION)

    # whether or not golang chaincode should be linked dynamically
    dynamicLink: false

  java:
    # This is an image based on java:openjdk-8 with addition compiler
    # tools added for java shim layer packaging.
    # This image is packed with shim layer libraries that are necessary
    # for Java chaincode runtime.
    runtime: $(DOCKER_NS)/fabric-javaenv:$(TWO_DIGIT_VERSION)

  node:
    # This is an image based on node:$(NODE_VER)-alpine
    runtime: $(DOCKER_NS)/fabric-nodeenv:$(TWO_DIGIT_VERSION)

  # List of directories to treat as external builders and launchers for
  # chaincode. The external builder detection processing will iterate over the
  # builders in the order specified below.
  externalBuilders:
    - name: ccaas_builder
      path: /opt/hyperledger/ccaas_builder
      propagateEnvironment:
      - CHAINCODE_AS_A_SERVICE_BUILDER_CONFIG
  # The maximum duration to wait for the chaincode build and install process
  # to complete.
  installTimeout: 8m0s

  # Timeout duration for starting up a container and waiting for Register
  # to come through.
  startuptimeout: 5m0s

  # Timeout duration for Invoke and Init calls to prevent runaway.
  # This timeout is used by all chaincodes in all the channels, including
  # system chaincodes.
  # Note that during Invoke, if the image is not available (e.g. being
  # cleaned up when in development environment), the peer will automatically
  # build the image, which might take more time. In production environment,
  # the chaincode image is unlikely to be deleted, so the timeout could be
  # reduced accordingly.
  executetimeout: 30s

  # There are 2 modes: "dev" and "net".
  # In dev mode, user runs the chaincode after starting peer from
  # command line on local machine.
  # In net mode, peer will run chaincode in a docker container.
  mode: net

  # keepalive in seconds. In situations where the communication goes through a
  # proxy that does not support keep-alive, this parameter will maintain connection
  # between peer and chaincode.
  # A value <= 0 turns keepalive off
  keepalive: 0

  # enabled system chaincodes
  system:
    _lifecycle: enable
    cscc: enable
    lscc: enable
    escc: enable
    vscc: enable
    qscc: enable

  # Logging section for the chaincode container
  logging:
    # Default level for all loggers within the chaincode container
    level:  info
    # Override default level for the 'shim' logger
    shim:   warning
    # Format for the chaincode container logs
    format: '%{color}%{time:2006-01-02 15:04:05.000 MST} [%{module}] %{shortfunc} -> %{level:.4s} %{id:03x}%{color:reset} %{message}'

###############################################################################
#
#    Ledger section - ledger configuration encompasses both the blockchain
#    and the state
#
###############################################################################
ledger:

  blockchain:
  snapshots:
    rootDir: {{ .FileSystemPath }}/snapshots

  state:
    # stateDatabase - options are "goleveldb", "CouchDB"
    # goleveldb - default state database stored in goleveldb.
    # CouchDB - store state database in CouchDB
    stateDatabase: goleveldb
    # Limit on the number of records to return per query
    totalQueryLimit: 100000
    couchDBConfig:
      # It is recommended to run CouchDB on the same server as the peer, and
      # not map the CouchDB container port to a server port in docker-compose.
      # Otherwise proper security must be provided on the connection between
      # CouchDB client (on the peer) and server.
      couchDBAddress: 127.0.0.1:5984
      # This username must have read and write authority on CouchDB
      username:
      # The password is recommended to pass as an environment variable
      # during start up (eg CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD).
      # If it is stored here, the file must be access control protected
      # to prevent unintended users from discovering the password.
      password:
      # Number of retries for CouchDB errors
      maxRetries: 3
      # Number of retries for CouchDB errors during peer startup.
      # The delay between retries doubles for each attempt.
      # Default of 10 retries results in 11 attempts over 2 minutes.
      maxRetriesOnStartup: 10
      # CouchDB request timeout (unit: duration, e.g. 20s)
      requestTimeout: 35s
      # Limit on the number of records per each CouchDB query
      # Note that chaincode queries are only bound by totalQueryLimit.
      # Internally the chaincode may execute multiple CouchDB queries,
      # each of size internalQueryLimit.
      internalQueryLimit: 1000
      # Limit on the number of records per CouchDB bulk update batch
      maxBatchUpdateSize: 1000
      # Warm indexes after every N blocks.
      # This option warms any indexes that have been
      # deployed to CouchDB after every N blocks.
      # A value of 1 will warm indexes after every block commit,
      # to ensure fast selector queries.
      # Increasing the value may improve write efficiency of peer and CouchDB,
      # but may degrade query response time.
      warmIndexesAfterNBlocks: 1
      # Create the _global_changes system database
      # This is optional.  Creating the global changes database will require
      # additional system resources to track changes and maintain the database
      createGlobalChangesDB: false
      # CacheSize denotes the maximum mega bytes (MB) to be allocated for the in-memory state
      # cache. Note that CacheSize needs to be a multiple of 32 MB. If it is not a multiple
      # of 32 MB, the peer would round the size to the next multiple of 32 MB.
      # To disable the cache, 0 MB needs to be assigned to the cacheSize.
      cacheSize: 64

  history:
    # enableHistoryDatabase - options are true or false
    # Indicates if the history of key updates should be stored.
    # All history 'index' will be stored in goleveldb, regardless if using
    # CouchDB or alternate database for the state.
    enableHistoryDatabase: true

  pvtdataStore:
    # the maximum db batch size for converting
    # the ineligible missing data entries to eligible missing data entries
    collElgProcMaxDbBatchSize: 5000
    # the minimum duration (in milliseconds) between writing
    # two consecutive db batches for converting the ineligible missing data entries to eligible missing data entries
    collElgProcDbBatchesInterval: 1000

###############################################################################
#
#    Operations section
#
###############################################################################
operations:
  # host and port for the operations server
  listenAddress: 127.0.0.1:9443

  # TLS configuration for the operations endpoint
  tls:
    # TLS enabled
    enabled: false

    # path to PEM encoded server certificate for the operations server
    cert:
      file:

    # path to PEM encoded server key for the operations server
    key:
      file:

    # most operations service endpoints require client authentication when TLS
    # is enabled. clientAuthRequired requires client certificate authentication
    # at the TLS layer to access all resources.
    clientAuthRequired: false

    # paths to PEM encoded ca certificates to trust for client authentication
    clientRootCAs:
      files: []

###############################################################################
#
#    Metrics section
#
###############################################################################
metrics:
  # metrics provider is one of statsd, prometheus, or disabled
  provider: disabled

  # statsd configuration
  statsd:
    # network type: tcp or udp
    network: udp

    # statsd server address
    address: 127.0.0.1:8125

    # the interval at which locally cached counters and gauges are pushed
    # to statsd; timings are pushed immediately
    writeInterval: 10s

    # prefix is prepended to all emitted statsd metrics
    prefix:
`
)

func EnrollPeerCertificates(
	peerInitOpts config.PeerInitOptions,
) error {
	peerID := peerInitOpts.ID
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	peerDir := filepath.Join(home, fmt.Sprintf("hlf-easy/peers/%s", peerID))
	err = os.MkdirAll(peerDir, 0755)
	if err != nil {
		log.Fatal(err)
	}
	if !peerInitOpts.Local {
		return errors.Errorf("not local provisioning is not implemented")
	}
	// init the certs
	caConfig, err := utils.GetCAConfig(peerInitOpts.CAName)
	if err != nil {
		return err
	}
	var ips []net.IP
	var dnsNames []string
	for _, host := range peerInitOpts.Hosts {
		// check if it's ip address
		ip := net.ParseIP(host)
		if ip != nil {
			ips = append(ips, ip)
		} else {
			dnsNames = append(dnsNames, host)
		}
	}
	// create peer tls cert
	tlsCert, tlsKey, err := certs.GenerateCertificate(
		certs.GenerateCertificateOptions{
			CommonName:       "peer",
			OrganizationUnit: []string{"peer"},
			IPAddresses:      ips,
			DNSNames:         dnsNames,
		},
		caConfig.TLSCACert,
		caConfig.TLSCAKey,
	)
	if err != nil {
		return err
	}

	// create peer cert
	peerCert, peerKey, err := certs.GenerateCertificate(
		certs.GenerateCertificateOptions{
			CommonName:       "peer",
			OrganizationUnit: []string{"peer"},
			IPAddresses:      []net.IP{},
			DNSNames:         []string{},
		},
		caConfig.CACert,
		caConfig.CAKey,
	)
	if err != nil {
		return err
	}
	tlsKeyBytes, err := utils.EncodePrivateKey(tlsKey)
	if err != nil {
		return err
	}
	signKeyBytes, err := utils.EncodePrivateKey(peerKey)
	if err != nil {
		return err
	}

	peerConfig := config.PeerConfig{
		TLSKey:    tlsKeyBytes,
		TLSCert:   utils.EncodeX509Certificate(tlsCert),
		SignKey:   signKeyBytes,
		SignCert:  utils.EncodeX509Certificate(peerCert),
		PeerID:    peerInitOpts.ID,
		TlsCACert: utils.EncodeX509Certificate(caConfig.TLSCACert),
		CaCert:    utils.EncodeX509Certificate(caConfig.CACert),
	}
	peerConfigBytes, err := json.MarshalIndent(peerConfig, "", "  ")
	if err != nil {
		return err
	}
	peerConfigFilePath := filepath.Join(peerDir, "config.json")
	err = os.WriteFile(peerConfigFilePath, peerConfigBytes, 0644)
	if err != nil {
		return err
	}

	// keystore key pem
	keyStoreDir := filepath.Join(peerDir, "keystore")
	err = os.MkdirAll(keyStoreDir, 0755)
	if err != nil {
		return err
	}
	signKeyFilePath := filepath.Join(keyStoreDir, "key.pem")
	err = os.WriteFile(signKeyFilePath, signKeyBytes, 0644)
	if err != nil {
		return err
	}

	// tlscacerts pem
	tlsCACertsDir := filepath.Join(peerDir, "tlscacerts")
	err = os.MkdirAll(tlsCACertsDir, 0755)
	if err != nil {
		return err
	}
	tlsCACertFilePath := filepath.Join(tlsCACertsDir, "cacert.pem")
	err = os.WriteFile(tlsCACertFilePath, utils.EncodeX509Certificate(caConfig.TLSCACert), 0644)
	if err != nil {
		return err
	}

	// cacerts pem
	cACertsDir := filepath.Join(peerDir, "cacerts")
	err = os.MkdirAll(cACertsDir, 0755)
	if err != nil {
		return err
	}
	caCertFilePath := filepath.Join(cACertsDir, "cacert.pem")
	err = os.WriteFile(caCertFilePath, utils.EncodeX509Certificate(caConfig.CACert), 0644)
	if err != nil {
		return err
	}

	// signcerts pem
	signCertsDir := filepath.Join(peerDir, "signcerts")
	err = os.MkdirAll(signCertsDir, 0755)
	if err != nil {
		return err
	}
	signCertFilePath := filepath.Join(signCertsDir, "cert.pem")
	err = os.WriteFile(signCertFilePath, utils.EncodeX509Certificate(peerCert), 0644)
	if err != nil {
		return err
	}

	// config.yaml
	configFilePath := filepath.Join(peerDir, "config.yaml")
	configYamlContent := `NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/cacert.pem
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/cacert.pem
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/cacert.pem
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/cacert.pem
    OrganizationalUnitIdentifier: orderer
`
	err = os.WriteFile(configFilePath, []byte(configYamlContent), 0644)
	if err != nil {
		return err
	}
	// write tls.key
	tlsKeyFilePath := filepath.Join(peerDir, "tls.key")
	err = os.WriteFile(tlsKeyFilePath, tlsKeyBytes, 0644)
	if err != nil {
		return err
	}

	// write tls.crt
	tlsCertFilePath := filepath.Join(peerDir, "tls.crt")
	err = os.WriteFile(tlsCertFilePath, utils.EncodeX509Certificate(tlsCert), 0644)
	if err != nil {
		return err
	}

	// write core.yaml based in the template
	tmpl, err := template.New("core.yaml").Funcs(sprig.HermeticTxtFuncMap()).Parse(coreYamlTemplate)
	if err != nil {
		return err
	}
	coreYamlFilePath := filepath.Join(peerDir, "core.yaml")
	coreYamlFile, err := os.Create(coreYamlFilePath)
	if err != nil {
		return err
	}
	defer coreYamlFile.Close()
	err = tmpl.Execute(coreYamlFile, struct {
		FileSystemPath string
	}{
		FileSystemPath: filepath.Join(peerDir, "data"),
	})
	if err != nil {
		return err
	}
	peerInitOptsBytes, err := json.Marshal(peerInitOpts)
	if err != nil {
		return err
	}
	initJsonPath := filepath.Join(peerDir, "init.json")
	err = os.WriteFile(initJsonPath, peerInitOptsBytes, 0644)
	if err != nil {
		return err
	}
	return nil
}
