<<<<<<< HEAD
Fabric CA User's Guide
======================

Fabric CA is a Certificate Authority for Hyperledger Fabric.

| It provides features such as:
| 1) registration of identities, or connects to LDAP as the user
  registry;
| 2) issuance of Enrollment Certificates (ECerts);
| 3) issuance of Transaction Certificates (TCerts), providing both
  anonymity and unlinkability when transacting on a Hyperledger Fabric
  blockchain;
| 4) certificate renewal and revocation.

Fabric CA consists of both a server and a client component as described
later in this document.

For developers interested in contributing to Fabric CA, see the `Fabric
CA repository <https://github.com/hyperledger/fabric-ca>`__ for more
information.


.. _Back to Top:

Table of Contents
-----------------

1. `Overview`_

2. `Getting Started`_

   1. `Prerequisites`_
   2. `Install`_
   3. `Explore the Fabric CA CLI`_

3. `File Formats`_

   1. `Fabric CA server's configuration file format`_
   2. `Fabric CA client's configuration file format`_

4. `Configuration Settings Precedence`_

5. `Fabric CA Server`_

   1. `Initializing the server`_
   2. `Starting the server`_
   3. `Configuring the database`_
   4. `Configuring LDAP`_
   5. `Setting up a cluster`_

6. `Fabric CA Client`_

   1. `Enrolling the bootstrap user`_
   2. `Registering a new identity`_
   3. `Enrolling a peer identity`_
   4. `Reenrolling an identity`_
   5. `Revoking a certificate or identity`_
   6. `Enabling TLS`_

7. `Appendix`_

Overview
--------

The diagram below illustrates how the Fabric CA server fits into the
overall Hyperledger Fabric architecture.

.. image:: ../images/fabric-ca.png

There are two ways of interacting with a Fabric CA server:
via the Fabric CA client or through one of the Fabric SDKs.
All communication to the Fabric CA server is via REST APIs.
See `fabric-ca/swagger/swagger-fabric-ca.json` for the swagger documentation
for these REST APIs.

The Fabric CA client or SDK may connect to a server in a cluster of Fabric CA
servers.   This is illustrated in the top right section of the diagram.
The client routes to an HA Proxy endpoint which load balances traffic to one
of the fabric-ca-server cluster members.
All Fabric CA servers in a cluster share the same database for
keeping track of users and certificates.  If LDAP is configured, the user
information is kept in LDAP rather than the database.

Getting Started
---------------

Prerequisites
~~~~~~~~~~~~~~~

-  Go 1.7+ installation or later
-  **GOPATH** environment variable is set correctly
- libtool and libtdhl-dev packages are installed

The following installs the libtool dependencies.

::

   # sudo apt install libtool libltdl-dev

For more information on libtool, see https://www.gnu.org/software/libtool.

For more information on libtdhr-dev, see https://www.gnu.org/software/libtool/manual/html_node/Using-libltdl.html.

Install
~~~~~~~

The following installs both the `fabric-ca-server` and `fabric-ca-client` commands.

::

    # go get -u github.com/hyperledger/fabric-ca/cmd/...

Start Server Natively
~~~~~~~~~~~~~~~~~~~~~

The following starts the `fabric-ca-server` with default settings.

::

    # fabric-ca-server start -b admin:adminpw

The `-b` option provides the enrollment ID and secret for a bootstrap
administrator.  A default configuration file named `fabric-ca-server-config.yaml`
is created in the local directory which can be customized.

Start Server via Docker
~~~~~~~~~~~~~~~~~~~~~~~

You can build and start the server via docker-compose as shown below.

::

    # cd $GOPATH/src/github.com/hyperledger/fabric-ca
    # make docker
    # cd docker/server
    # docker-compose up -d

The hyperledger/fabric-ca docker image contains both the fabric-ca-server and
the fabric-ca-client.

Explore the Fabric CA CLI
~~~~~~~~~~~~~~~~~~~~~~~~~~~

This section simply provides the usage messages for the Fabric CA server and client
for convenience.  Additional usage information is provided in following sections.

The following shows the Fabric CA server usage message.

::

    Hyperledger Fabric Certificate Authority Server

    Usage:
      fabric-ca-server [command]

    Available Commands:
      init        Initialize the fabric-ca server
      start       Start the fabric-ca server

    Flags:
          --address string                  Listening address of fabric-ca-server (default "0.0.0.0")
      -b, --boot string                     The user:pass for bootstrap admin which is required to build default config file
          --ca.certfile string              PEM-encoded CA certificate file (default "ca-cert.pem")
          --ca.keyfile string               PEM-encoded CA key file (default "ca-key.pem")
      -c, --config string                   Configuration file (default "fabric-ca-server-config.yaml")
          --csr.cn string                   The common name field of the certificate signing request to a parent fabric-ca-server
          --csr.serialnumber string         The serial number in a certificate signing request to a parent fabric-ca-server
          --db.datasource string            Data source which is database specific (default "fabric-ca-server.db")
          --db.tls.certfiles string         PEM-encoded comma separated list of trusted certificate files (e.g. root1.pem, root2.pem)
          --db.tls.client.certfile string   PEM-encoded certificate file when mutual authentication is enabled
          --db.tls.client.keyfile string    PEM-encoded key file when mutual authentication is enabled
          --db.tls.enabled                  Enable TLS for client connection
          --db.type string                  Type of database; one of: sqlite3, postgres, mysql (default "sqlite3")
      -d, --debug                           Enable debug level logging
          --ldap.enabled                    Enable the LDAP client for authentication and attributes
          --ldap.groupfilter string         The LDAP group filter for a single affiliation group (default "(memberUid=%s)")
          --ldap.url string                 LDAP client URL of form ldap://adminDN:adminPassword@host[:port]/base
          --ldap.userfilter string          The LDAP user filter to use when searching for users (default "(uid=%s)")
      -p, --port int                        Listening port of fabric-ca-server (default 7054)
          --registry.maxenrollments int     Maximum number of enrollments; valid if LDAP not enabled
          --tls.certfile string             PEM-encoded TLS certificate file for server's listening port (default "ca-cert.pem")
          --tls.enabled                     Enable TLS on the listening port
          --tls.keyfile string              PEM-encoded TLS key for server's listening port (default "ca-key.pem")
      -u, --url string                      URL of the parent fabric-ca-server

    Use "fabric-ca-server [command] --help" for more information about a command.

The following shows the Fabric CA client usage message:

::

    # fabric-ca-client
    Hyperledger Fabric Certificate Authority Client

    Usage:
      fabric-ca-client [command]

    Available Commands:
      enroll      Enroll user
      getcacert   Get CA certificate chain
      reenroll    Reenroll user
      register    Register user
      revoke      Revoke user

    Flags:
      -c, --config string                Configuration file (default "$HOME/.fabric-ca-client/fabric-ca-client-config.yaml")
          --csr.cn string                The common name field of the certificate signing request to a parent fabric-ca-server
          --csr.serialnumber string      The serial number in a certificate signing request to a parent fabric-ca-server
      -d, --debug                        Enable debug level logging
          --enrollment.hosts string      Comma-separated host list
          --enrollment.label string      Label to use in HSM operations
          --enrollment.profile string    Name of the signing profile to use in issuing the certificate
          --id.affiliation string        The identity's affiliation
          --id.attr string               Attributes associated with this identity (e.g. hf.Revoker=true)
          --id.maxenrollments int        The maximum number of times the secret can be reused to enroll
          --id.name string               Unique name of the identity
          --id.secret string             The enrollment secret for the identity being registered
          --id.type string               Type of identity being registered (e.g. 'peer, app, user')
      -M, --mspdir string                Membership Service Provider directory (default "msp")
      -m, --myhost string                Hostname to include in the certificate signing request during enrollment (default "$HOSTNAME")
          --tls.certfiles string         PEM-encoded comma separated list of trusted certificate files (e.g. root1.pem, root2.pem)
          --tls.client.certfile string   PEM-encoded certificate file when mutual authentication is enabled
          --tls.client.keyfile string    PEM-encoded key file when mutual authentication is enabled
          --tls.enabled                  Enable TLS for client connection
      -u, --url string                   URL of fabric-ca-server (default "http://localhost:7054")

    Use "fabric-ca-client [command] --help" for more information about a command.


`Back to Top`_

File Formats
------------

Fabric CA server's configuration file format
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

A configuration file can be provided to the server using the ``-c`` or ``--config``
option. If the config option is used and the specified file doesn't exist,
a default configuration file (like the one shown below) will be created in the
specified location. However, if no config option was used, it will be created in
the server's home directory (see `Fabric CA Server <#server>`__ section more info).

::

    # Server's listening port (default: 7054)
    port: 7054

    # Enables debug logging (default: false)
    debug: false

    #############################################################################
    #  TLS section for the server's listening port
    #############################################################################
    tls:
      # Enable TLS (default: false)
      enabled: false
      certfile: ca-cert.pem
      keyfile: ca-key.pem

    #############################################################################
    #  The CA section contains the key and certificate files used when
    #  issuing enrollment certificates (ECerts) and transaction
    #  certificates (TCerts).
    #############################################################################
    ca:
      # Certificate file (default: ca-cert.pem)
      certfile: ca-cert.pem
      # Key file (default: ca-key.pem)
      keyfile: ca-key.pem

    #############################################################################
    #  The registry section controls how the fabric-ca-server does two things:
    #  1) authenticates enrollment requests which contain a username and password
    #     (also known as an enrollment ID and secret).
    #  2) once authenticated, retrieves the identity's attribute names and
    #     values which the fabric-ca-server optionally puts into TCerts
    #     which it issues for transacting on the Hyperledger Fabric blockchain.
    #     These attributes are useful for making access control decisions in
    #     chaincode.
    #  There are two main configuration options:
    #  1) The fabric-ca-server is the registry
    #  2) An LDAP server is the registry, in which case the fabric-ca-server
    #     calls the LDAP server to perform these tasks.
    #############################################################################
    registry:
      # Maximum number of times a password/secret can be reused for enrollment
      # (default: 0, which means there is no limit)
      maxEnrollments: 0

      # Contains user information which is used when LDAP is disabled
      identities:
         - name: <<<ADMIN>>>
           pass: <<<ADMINPW>>>
           type: client
           affiliation: ""
           attrs:
              hf.Registrar.Roles: "client,user,peer,validator,auditor,ca"
              hf.Registrar.DelegateRoles: "client,user,validator,auditor"
              hf.Revoker: true
              hf.IntermediateCA: true

    #############################################################################
    #  Database section
    #  Supported types are: "sqlite3", "postgres", and "mysql".
    #  The datasource value depends on the type.
    #  If the type is "sqlite3", the datasource value is a file name to use
    #  as the database store.  Since "sqlite3" is an embedded database, it
    #  may not be used if you want to run the fabric-ca-server in a cluster.
    #  To run the fabric-ca-server in a cluster, you must choose "postgres"
    #  or "mysql".
    #############################################################################
    db:
      type: sqlite3
      datasource: fabric-ca-server.db
      tls:
          enabled: false
          certfiles: db-server-cert.pem
          client:
            certfile: db-client-cert.pem
            keyfile: db-client-key.pem

    #############################################################################
    #  LDAP section
    #  If LDAP is enabled, the fabric-ca-server calls LDAP to:
    #  1) authenticate enrollment ID and secret (i.e. username and password)
    #     for enrollment requests
    #  2) To retrieve identity attributes
    #############################################################################
    ldap:
       # Enables or disables the LDAP client (default: false)
       enabled: false
       # The URL of the LDAP server
       url: ldap://<adminDN>:<adminPassword>@<host>:<port>/<base>
       tls:
          certfiles: ldap-server-cert.pem
          client:
             certfile: ldap-client-cert.pem
             keyfile: ldap-client-key.pem

    #############################################################################
    #  Affiliation section
    #############################################################################
    affiliations:
       org1:
          - department1
          - department2
       org2:
          - department1

    #############################################################################
    #  Signing section
    #############################################################################
    signing:
        profiles:
          ca:
             usage:
               - cert sign
             expiry: 8000h
             caconstraint:
               isca: true
        default:
          usage:
            - cert sign
          expiry: 8000h

    ###########################################################################
    #  Certificate Signing Request section for generating the CA certificate
    ###########################################################################
    csr:
       cn: fabric-ca-server
       names:
          - C: US
            ST: "North Carolina"
            L:
            O: Hyperledger
            OU: Fabric
       hosts:
         - <<<MYHOST>>>
       ca:
          pathlen:
          pathlenzero:
          expiry:

    #############################################################################
    #  Crypto section configures the crypto primitives used for all
    #############################################################################
    crypto:
      software:
         hash_family: SHA2
         security_level: 256
         ephemeral: false
         key_store_dir: keys

Fabric CA client's configuration file format
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

A configuration file can be provided to the client using the ``-c`` or ``--config``
option. If the config option is used and the specified file doesn't exist,
a default configuration file (like the one shown below) will be created in the
specified location. However, if no config option was used, it will be created in
the client's home directory (see `Fabric CA Client <#client>`__ section more info).

::

    #############################################################################
    # Client Configuration
    #############################################################################

    # URL of the fabric-ca-server (default: http://localhost:7054)
    URL: http://localhost:7054

    # Membership Service Provider (MSP) directory
    # When the client is used to enroll a peer or an orderer, this field must be
    # set to the MSP directory of the peer/orderer
    MSPDir:

    #############################################################################
    #    TLS section for secure socket connection
    #############################################################################
    tls:
      # Enable TLS (default: false)
      enabled: false
      certfiles:   # Comma Separated (e.g. root.pem, root2.pem)
      client:
        certfile:
        keyfile:

    #############################################################################
    #  Certificate Signing Request section for generating the CSR for
    #  an enrollment certificate (ECert)
    #############################################################################
    csr:
      cn: <<<ENROLLMENT_ID>>>
      names:
        - C: US
          ST: "North Carolina"
          L:
          O: Hyperledger
          OU: Fabric
      hosts:
       - <<<MYHOST>>>
      ca:
        pathlen:
        pathlenzero:
        expiry:

    #############################################################################
    #  Registration section used to register a new user with fabric-ca server
    #############################################################################
    id:
      name:
      type:
      affiliation:
      attributes:
        - name:
          value:

    #############################################################################
    #  Enrollment section used to enroll a user with fabric-ca server
    #############################################################################
    enrollment:
      hosts:
      profile:
      label:

`Back to Top`_

Configuration Settings Precedence
---------------------------------

The Fabric CA provides 3 ways to configure settings on the fabric-ca-server
and fabric-ca-client. The precedence order is:

1. CLI flags
2. Environment variables
3. Configuration file

In the remainder of this document, we refer to making changes to
configuration files. However, configuration file changes can be
overridden through environment variables or CLI flags.

For example, if we have the following in the client configuration file:

::

    tls:
      # Enable TLS (default: false)
      enabled: false

      # TLS for the client's listenting port (default: false)
      certfiles:   # Comma Separated (e.g. root.pem, root2.pem)
      client:
        certfile: cert.pem
        keyfile:

The following environment variable may be used to override the ``cert.pem``
setting in the configuration file:

``export FABRIC_CA_CLIENT_TLS_CLIENT_CERTFILE=cert2.pem``

If we wanted to override both the environment variable and configuration
file, we can use a command line flag.

``fabric-ca-client enroll --tls.client.certfile cert3.pem``

The same approach applies to fabric-ca-server, except instead of using
``FABIRC_CA_CLIENT`` as the prefix to environment variables,
``FABRIC_CA_SERVER`` is used.

.. _server:


A word on file paths
--------------------
All the properties in the Fabric CA server and client configuration file,
that specify file names support both relative and absolute paths.
Relative paths are relative to the config directory, where the
configuration file is located. For example, if the config directory is
``~/config`` and the tls section is as shown below, the Fabric CA server
or client will look for the ``root.pem`` file in the ``~/config``
directory, ``cert.pem`` file in the ``~/config/certs`` directory and the
``key.pem`` file in the ``/abs/path`` directory

::

    tls:
      enabled: true
      certfiles:   root.pem
      client:
        certfile: certs/cert.pem
        keyfile: /abs/path/key.pem



Fabric CA Server
----------------

This section describes the Fabric CA server.

You may initialize the Fabric CA server before starting it if you prefer.
This provides an opportunity for you to generate a default configuration
file but to review and customize its settings before starting it.

| The fabric-ca-server's home directory is determined as follows:
| - if the ``FABRIC_CA_SERVER_HOME`` environment variable is set, use
  its value;
| - otherwise, if ``FABRIC_CA_HOME`` environment variable is set, use
  its value;
| - otherwise, if the ``CA_CFG_PATH`` environment variable is set, use
  its value;
| - otherwise, use current working directory.

For the remainder of this server section, we assume that you have set
the ``FABRIC_CA_HOME`` environment variable to
``$HOME/fabric-ca/server``.

The instructions below assume that the server configuration file exists
in the server's home directory.

.. _initialize:

Initializing the server
~~~~~~~~~~~~~~~~~~~~~~~

Initialize the Fabric CA server as follows:

::

    # fabric-ca-server init -b admin:adminpw

The ``-b`` (bootstrap user) option is required for initialization. At
least one bootstrap user is required to start the fabric-ca-server. The
server configuration file contains a Certificate Signing Request (CSR)
section that can be configured. The following is a sample CSR.

If you are going to connect to the fabric-ca-server remotely over TLS,
replace "localhost" in the CSR section below with the hostname where you
will be running your fabric-ca-server.

.. _csr-fields:

::

    cn: localhost
    key:
        algo: ecdsa
        size: 256
    names:
      - C: US
        ST: "North Carolina"
        L:
        O: Hyperledger
        OU: Fabric

All of the fields above pertain to the X.509 signing key and certificate which
is generated by the ``fabric-ca-server init``.  This corresponds to the
``ca.certfile`` and ``ca.keyfile`` files in the server's configuration file.
The fields are as follows:

-  **cn** is the Common Name
-  **key** specifies the algorithm and key size as described below
-  **O** is the organization name
-  **OU** is the organizational unit
-  **L** is the location or city
-  **ST** is the state
-  **C** is the country

If custom values for the CSR are required, you may customize the configuration
file, delete the files specified by the ``ca.certfile`` and ``ca-keyfile``
configuration items, and then run the ``fabric-ca-server init -b admin:adminpw``
command again.

The ``fabric-ca-server init`` command generates a self-signed CA certificate
unless the ``-u <parent-fabric-ca-server-URL>`` option is specified.
If the ``-u`` is specified, the server's CA certificate is signed by the
parent fabric-ca-server.  The ``fabric-ca-server init`` command also
generates a default configuration file named **fabric-ca-server-config.yaml**
in the server's home directory.

Algorithms and key sizes

The CSR can be customized to generate X.509 certificates and keys that
support both RSA and Elliptic Curve (ECDSA). The following setting is an
example of the implementation of Elliptic Curve Digital Signature
Algorithm (ECDSA) with curve ``prime256v1`` and signature algorithm
``ecdsa-with-SHA256``:

::

    key:
       algo: ecdsa
       size: 256

The choice of algorithm and key size are based on security needs.

Elliptic Curve (ECDSA) offers the following key size options:

+--------+--------------+-----------------------+
| size   | ASN1 OID     | Signature Algorithm   |
+========+==============+=======================+
| 256    | prime256v1   | ecdsa-with-SHA256     |
+--------+--------------+-----------------------+
| 384    | secp384r1    | ecdsa-with-SHA384     |
+--------+--------------+-----------------------+
| 521    | secp521r1    | ecdsa-with-SHA512     |
+--------+--------------+-----------------------+

RSA offers the following key size options:

+--------+------------------+---------------------------+
| size   | Modulus (bits)   | Signature Algorithm       |
+========+==================+===========================+
| 2048   | 2048             | sha256WithRSAEncryption   |
+--------+------------------+---------------------------+
| 4096   | 4096             | sha512WithRSAEncryption   |
+--------+------------------+---------------------------+

Starting the server
~~~~~~~~~~~~~~~~~~~

Start the Fabric CA server as follows:

::

    # fabric-ca-server start -b <admin>:<adminpw>

If the server has not been previously initialized, it will initialize
itself as it starts for the first time.  During this initialization, the
server will generate the ca-cert.pem and ca-key.pem files if they don't
yet exist and will also create a default configuration file if it does
not exist.  See the `Initialize the Fabric CA server <#initialize>`__ section.

Unless the fabric-ca-server is configured to use LDAP, it must be
configured with at least one pre-registered bootstrap user to enable you
to register and enroll other identities. The ``-b`` option specifies the
name and password for a bootstrap user.

A different configuration file may be specified with the ``-c`` option
as shown below.

::

    # fabric-ca-server start -c <path-to-config-file> -b <admin>:<adminpw>

To cause the fabric-ca-server to listen on ``https`` rather than
``http``, set ``tls.enabled`` to ``true``.

To limit the number of times that the same secret (or password) can be
used for enrollment, set the ``registry.maxEnrollments`` in the configuration
file to the appropriate value. If you set the value to 1, the fabric-ca
server allows passwords to only be used once for a particular enrollment
ID. If you set the value to 0, the fabric-ca-server places no limit on
the number of times that a secret can be reused for enrollment. The
default value is 0.

The fabric-ca-server should now be listening on port 7054.

You may skip to the `Fabric CA Client <#fabric-ca-client>`__ section if
you do not want to configure the fabric-ca-server to run in a cluster or
to use LDAP.

Configuring the database
~~~~~~~~~~~~~~~~~~~~~~~~

This section describes how to configure the fabric-ca-server to connect
to Postgres or MySQL databases. The default database is SQLite and the
default database file is ``fabric-ca-server.db`` in the Fabric CA
server's home directory.

If you don't care about running the fabric-ca-server in a cluster, you
may skip this section; otherwise, you must configure either Postgres or
MySQL as described below.

Postgres
^^^^^^^^^^

The following sample may be added to the server's configuration file in
order to connect to a Postgres database. Be sure to customize the
various values appropriately.

::

    db:
      type: postgres
      datasource: host=localhost port=5432 user=Username password=Password dbname=fabric-ca-server sslmode=verify-full

Specifying *sslmode* configures the type of SSL authentication. Valid
values for sslmode are:

|

+----------------+----------------+
| Mode           | Description    |
+================+================+
| disable        | No SSL         |
+----------------+----------------+
| require        | Always SSL     |
|                | (skip          |
|                | verification)  |
+----------------+----------------+
| verify-ca      | Always SSL     |
|                | (verify that   |
|                | the            |
|                | certificate    |
|                | presented by   |
|                | the server was |
|                | signed by a    |
|                | trusted CA)    |
+----------------+----------------+
| verify-full    | Same as        |
|                | verify-ca AND  |
|                | verify that    |
|                | the            |
|                | certificate    |
|                | presented by   |
|                | the server was |
|                | signed by a    |
|                | trusted CA and |
|                | the server     |
|                | host name      |
|                | matches the    |
|                | one in the     |
|                | certificate    |
+----------------+----------------+

|

If you would like to use TLS, then the ``db.tls`` section in the fabric-ca-server
configuration file must be specified. If SSL client authentication is enabled
on the Postgres server, then the client certificate and key file must also be
specified in the ``db.tls.client`` section. The following is an example
of the ``db.tls`` section:

::

    db:
      ...
      tls:
          enabled: true
          certfiles: db-server-cert.pem
          client:
                certfile: db-client-cert.pem
                keyfile: db-client-key.pem

| **certfiles** - Comma separated list of PEM-encoded trusted root certificate files.
| **certfile** and **keyfile** - PEM-encoded certificate and key files that are used by the Fabric CA server to communicate securely with the Postgres server

MySQL
^^^^^^^

The following sample may be added to the fabric-ca-server config file in
order to connect to a MySQL database. Be sure to customize the various
values appropriately.

::

    db:
      type: mysql
      datasource: root:rootpw@tcp(localhost:3306)/fabric-ca?parseTime=true&tls=custom

If connecting over TLS to the MySQL server, the ``db.tls.client``
section is also required as described in the **Postgres** section above.

Configuring LDAP
~~~~~~~~~~~~~~~~

The fabric-ca-server can be configured to read from an LDAP server.

In particular, the fabric-ca-server may connect to an LDAP server to do
the following:

-  authenticate a user prior to enrollment
-  retrieve a user's attribute values which are used for authorization.

Modify the LDAP section of the server's configuration file to configure the
fabric-ca-server to connect to an LDAP server.

::

    ldap:
       # Enables or disables the LDAP client (default: false)
       enabled: false
       # The URL of the LDAP server
       url: <scheme>://<adminDN>:<adminPassword>@<host>:<port>/<base>
       userfilter: filter

| where:
| \* ``scheme`` is one of *ldap* or *ldaps*;
| \* ``adminDN`` is the distinquished name of the admin user;
| \* ``pass`` is the password of the admin user;
| \* ``host`` is the hostname or IP address of the LDAP server;
| \* ``port`` is the optional port number, where default 389 for *ldap*
  and 636 for *ldaps*;
| \* ``base`` is the optional root of the LDAP tree to use for searches;
| \* ``filter`` is a filter to use when searching to convert a login
  user name to a distinquished name. For example, a value of
  ``(uid=%s)`` searches for LDAP entries with the value of a ``uid``
  attribute whose value is the login user name. Similarly,
  ``(email=%s)`` may be used to login with an email address.

The following is a sample configuration section for the default settings
for the OpenLDAP server whose docker image is at
``https://github.com/osixia/docker-openldap``.

::

    ldap:
       enabled: true
       url: ldap://cn=admin,dc=example,dc=org:admin@localhost:10389/dc=example,dc=org
       userfilter: (uid=%s)

See ``FABRIC_CA/scripts/run-ldap-tests`` for a script which starts an
OpenLDAP docker image, configures it, runs the LDAP tests in
``FABRIC_CA/cli/server/ldap/ldap_test.go``, and stops the OpenLDAP
server.

When LDAP is configured, enrollment works as follows:


-  The fabric-ca-client or client SDK sends an enrollment request with a
   basic authorization header.
-  The fabric-ca-server receives the enrollment request, decodes the
   user name and password in the authorization header, looks up the DN (Distinquished
   Name) associated with the user name using the "userfilter" from the
   configuration file, and then attempts an LDAP bind with the user's
   password. If the LDAP bind is successful, the enrollment processing is
   authorized and can proceed.

When LDAP is configured, attribute retrieval works as follows:


-  A client SDK sends a request for a batch of tcerts **with one or more
   attributes** to the fabric-ca-server.
-  The fabric-ca-server receives the tcert request and does as follows:

   -  extracts the enrollment ID from the token in the authorization
      header (after validating the token);
   -  does an LDAP search/query to the LDAP server, requesting all of
      the attribute names received in the tcert request;
   -  the attribute values are placed in the tcert as normal.

Setting up a cluster
~~~~~~~~~~~~~~~~~~~~

You may use any IP sprayer to load balance to a cluster of fabric-ca
servers. This section provides an example of how to set up Haproxy to
route to a fabric-ca-server cluster. Be sure to change hostname and port
to reflect the settings of your fabric-ca servers.

haproxy.conf

::

    global
          maxconn 4096
          daemon

    defaults
          mode http
          maxconn 2000
          timeout connect 5000
          timeout client 50000
          timeout server 50000

    listen http-in
          bind *:7054
          balance roundrobin
          server server1 hostname1:port
          server server2 hostname2:port
          server server3 hostname3:port


Node: If using TLS, need to use ``mode tcp``.

`Back to Top`_

.. _client:

Fabric CA Client
----------------

This section describes how to use the fabric-ca-client command.

| The fabric-ca-client's home directory is determined as follows:
| - if the ``FABRIC_CA_CLIENT_HOME`` environment variable is set, use
  its value;
| - otherwise, if the ``FABRIC_CA_HOME`` environment variable is set,
  use its value;
| - otherwise, if the ``CA_CFG_PATH`` environment variable is set, use
  its value;
| - otherwise, use ``$HOME/.fabric-ca-client``.


The instructions below assume that the client configuration file exists
in the client's home directory.

Enrolling the bootstrap user
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

First, if needed, customize the CSR (Certificate Signing Request) section
in the client configuration file. Note that ``csr.cn`` field must be set
to the ID of the bootstrap user. Default CSR values are shown below:

::

    csr:
      cn: <<enrollment ID>>
      key:
        algo: ecdsa
        size: 256
      names:
        - C: US
          ST: North Carolina
          L:
          O: Hyperledger Fabric
          OU: Fabric CA
      hosts:
       - <<hostname of the fabric-ca-client>>
      ca:
        pathlen:
        pathlenzero:
        expiry:

See `CSR fields <#csr-fields>`__ for description of the fields.

Then run ``fabric-ca-client enroll`` command to enroll the user. For example,
following command enrolls an user whose ID is **admin** and password is **adminpw**
by calling fabric-ca-server that is running locally at 7054 port.

::

    # export FABRIC_CA_CLIENT_HOME=$HOME/fabric-ca/clients/admin
    # fabric-ca-client enroll -u http://admin:adminpw@localhost:7054

The enroll command stores an enrollment certificate (ECert), corresponding private key and CA
certificate chain PEM files in the subdirectories of the fabric-ca-client's ``msp`` directory.
You will see messages indicating where the PEM files are stored.

Registering a new identity
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

The user performing the register request must be currently enrolled, and
must also have the proper authority to register the type of user being
registered.

In particular, two authorization checks are made by the fabric-ca-server
during registration as follows:

 1. The invoker's identity must have the "hf.Registrar.Roles" attribute with a
    comma-separated list of values where one of the value equals the type of
    identity being registered; for example, if the invoker's identity has the
    "hf.Registrar.Roles" attribute with a value of "peer,app,user", the invoker
    can register identities of type peer, app, and user, but not orderer.

 2. The affiliation of the invoker's identity must be equal to or a prefix of
    the affiliation of the identity being registered.  For example, an invoker
    with an affiliation of "a.b" may register an identity with an affiliation
    of "a.b.c" but may not register an identity with an affiliation of "a.c".

The following command uses the **admin** user's credentials to register a new
identity with an enrollment id of "admin2", a type of "user", an affiliation of
"org1.department1", and an attribute named "hf.Revoker" with a value of "true".

::

    # export FABRIC_CA_CLIENT_HOME=$HOME/fabric-ca/clients/admin
    # fabric-ca-client register --id.name admin2 --id.type user --id.affiliation org1.department1 --id.attr hf.Revoker=true

The password, also known as the enrollment secret, is printed.
This password is required to enroll the user.
This allows an administrator to register an identity and to then give the
enrollment ID and secret to someone else to enroll the identity.

You may set default values for any of the fields used in the register command
by editing the client's configuration file.  For example, suppose the configuration
file contains the following:

::

    id:
      name:
      type: user
      affiliation: org1.department1
      attributes:
        - name: hf.Revoker
          value: true
        - name: anotherAttrName
          value: anotherAttrValue

The following command would then register a new identity with an enrollment id of
"admin3" which it takes from the command line, and the remainder is taken from the
config file including a type of "user", an affiliation of "org1.department1", and two attributes:
"hf.Revoker" with a value of "true" and "anotherAttrName" with a value of "anotherAttrValue".

::

    # export FABRIC_CA_CLIENT_HOME=$HOME/fabric-ca/clients/admin
    # fabric-ca-client register --id.name admin3

To register a user with multiple attributes requires specifying all attribute names and values
in the configuration file as shown above.

Next, let's register a peer identity which will be used to enroll the peer in the following section.
The following command registers the **peer1** identity.  Note that we choose to specify our own
password (or secret) rather than letting the server generate one for us.

::

    # export FABRIC_CA_CLIENT_HOME=$HOME/fabric-ca/clients/admin
    # fabric-ca-client register --id.name peer1 --id.type peer --id.affiliation org1.department1 --id.secret peer1pw

Enrolling a Peer Identity
~~~~~~~~~~~~~~~~~~~~~~~~~

Now that you have successfully registered a peer identity, you may now
enroll the peer given the enrollment ID and secret (i.e. the *password*
from the previous section).  This is similar to enrolling the bootstrap user
except that we also demonstrate how to use the "-M" option to populate the
Hyperledger Fabric MSP (Membership Service Provider) directory structure.

The following command enrolls peer1.
Be sure to replace the value of the "-M" option with the path to your
peer's MSP directory which is the
'mspConfigPath' setting in the peer's core.yaml file.
You may also set the FABRIC_CA_CLIENT_HOME to the home directory of your peer.

::

    # export FABRIC_CA_CLIENT_HOME=$HOME/fabric-ca/clients/peer1
    # fabric-ca-client enroll -u http://peer1:peer1pw@localhost:7054 -M $FABRIC_CA_CLIENT_HOME/msp

Enrolling an orderer is the same, except the path to the MSP directory is
the 'LocalMSPDir' setting in your orderer's orderer.yaml file.

Getting a CA certificate chain from another fabric-ca-server
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

In general, the cacerts directory of the MSP directory must contain the certificate authority chains
of other certificate authorities, representing all of the roots of trust for the peer.

The ``fabric-ca-client getcacerts`` command is used to retrieve these certificate chains from other
fabric-ca-server instances.

For example, the following will start a second fabric-ca-server on localhost
listening on port 7055 with a name of "CA2".  This represents a completely separate
root of trust and would be managed by a different member on the blockchain.

::

    # export FABRIC_CA_SERVER_HOME=$HOME/ca2
    # fabric-ca-server start -b admin:ca2pw -p 7055 -n CA2

The following command will install CA2's certificate chain into peer1's MSP directory.

::

    # export FABRIC_CA_CLIENT_HOME=$HOME/fabric-ca/clients/peer1
    # fabric-ca-client getcacert -u http://localhost:7055 -M $FABRIC_CA_CLIENT_HOME/msp

Reenrolling an Identity
~~~~~~~~~~~~~~~~~~~~~~~

Suppose your enrollment certificate is about to expire or has been compromised.
You can issue the reenroll command to renew your enrollment certificate as follows.

::

    # export FABRIC_CA_CLIENT_HOME=$HOME/fabric-ca/clients/peer1
    # fabric-ca-client reenroll

Revoking a certificate or identity
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

In order to revoke a certificate or user, the calling identity must have
the ``hf.Revoker`` attribute.  The revoking identity can only revoke a
certificate or user that has an affiliation that is equal to or prefixed
by the revoking identity's affiliation.

For example, a revoker with affiliation bank.bank\_1 can revoke user
with bank.bank1.dep1 but can't revoke bank.bank2.

You may revoke a specific certificate by specifying its AKI (Authority
Key Identifier) and its serial number as follows:

::

    fabric-ca-client revoke -a xxx -s yyy -r <reason>

The following command disables a user's identity and also revokes all of
the certificates associated with the identity. All future requests
received by the fabric-ca-server from this identity will be rejected.

::

    fabric-ca-client revoke -e <enrollment_id> -r <reason>

The following are the supported reasons for revoking that can be
specified using ``-r`` flag.

| **Reasons:**
| - unspecified
| - keycompromise
| - cacompromise
| - affiliationchange
| - superseded
| - cessationofoperation
| - certificatehold
| - removefromcrl
| - privilegewithdrawn
| - aacompromise

The bootstrap admin can revoke **peer1**'s identity as follows:

::

    # export FABRIC_CA_CLIENT_HOME=$HOME/fabric-ca/clients/admin
    # fabric-ca-client revoke -e peer1

Enabling TLS
~~~~~~~~~~~~

This section describes in more detail how to configure TLS for a
fabric-ca-client.

The following sections may be configured in the ``fabric-ca-client-config.yaml``.

::

    tls:
      # Enable TLS (default: false)
      enabled: true
      certfiles: root.pem   # Comma Separated (e.g. root.pem,root2.pem)
      client:
        certfile: tls_client-cert.pem
        keyfile: tls_client-key.pem

The **certfiles** option is the set of root certificates trusted by the
client. This will typically just be the root fabric-ca-server's
certificate found in the server's home directory in the **ca-cert.pem**
file.

The **client** option is required only if mutual TLS is configured on
the server.

`Back to Top`_

Appendix
--------

Postgres SSL Configuration
~~~~~~~~~~~~~~~~~~~~~~~~~~

**Basic instructions for configuring SSL on the Postgres server:**

1. In postgresql.conf, uncomment SSL and set to "on" (SSL=on)

2. Place certificate and key files in the Postgres data directory.

Instructions for generating self-signed certificates for:
https://www.postgresql.org/docs/9.5/static/ssl-tcp.html

Note: Self-signed certificates are for testing purposes and should not
be used in a production environment

**Postgres Server - Require Client Certificates**

1. Place certificates of the certificate authorities (CAs) you trust in the file root.crt in the Postgres data directory

2. In postgresql.conf, set "ssl\_ca\_file" to point to the root cert of client (CA cert)

3. Set the clientcert parameter to 1 on the appropriate hostssl line(s) in pg\_hba.conf.

For more details on configuring SSL on the Postgres server, please refer
to the following Postgres documentation:
https://www.postgresql.org/docs/9.4/static/libpq-ssl.html

MySQL SSL Configuration
~~~~~~~~~~~~~~~~~~~~~~~

On MySQL 5.7.X, certain modes affect whether the server permits '0000-00-00' as a valid date.
It might be necessary to relax the modes that MySQL server uses. We want to allow
the server to be able to accept zero date values.

Please refer to the following MySQL documentation on different modes available
and select the appropriate settings for the specific version of MySQL that is
being used.

https://dev.mysql.com/doc/refman/5.7/en/sql-mode.html

**Basic instructions for configuring SSL on MySQL server:**

1. Open or create my.cnf file for the server. Add or un-comment the
   lines below in the [mysqld] section. These should point to the key and
   certificates for the server, and the root CA cert.

   Instructions on creating server and client side certficates:
   http://dev.mysql.com/doc/refman/5.7/en/creating-ssl-files-using-openssl.html

   [mysqld] ssl-ca=ca-cert.pem ssl-cert=server-cert.pem ssl-key=server-key.pem

   Can run the following query to confirm SSL has been enabled.

   mysql> SHOW GLOBAL VARIABLES LIKE 'have\_%ssl';

   Should see:

   +----------------+----------------+
   | Variable_name  | Value          |
   +================+================+
   | have_openssl   | YES            |
   +----------------+----------------+
   | have_ssl       | YES            |
   +----------------+----------------+

2. After the server-side SSL configuration is finished, the next step is
   to create a user who has a privilege to access the MySQL server over
   SSL. For that, log in to the MySQL server, and type:

   mysql> GRANT ALL PRIVILEGES ON *.* TO 'ssluser'@'%' IDENTIFIED BY
   'password' REQUIRE SSL; mysql> FLUSH PRIVILEGES;

   If you want to give a specific ip address from which the user will
   access the server change the '%' to the specific ip address.

**MySQL Server - Require Client Certificates**

Options for secure connections are similar to those used on the server side.

-  ssl-ca identifies the Certificate Authority (CA) certificate. This
   option, if used, must specify the same certificate used by the
   server.
-  ssl-cert identifies MySQL server's certificate.
-  ssl-key identifies MySQL server's private key.

Suppose that you want to connect using an account that has no special
encryption requirements or was created using a GRANT statement that
includes the REQUIRE SSL option. As a recommended set of
secure-connection options, start the MySQL server with at least
--ssl-cert and --ssl-key, and invoke the fabric-ca-server with
``db.tls.certfiles`` option set in the Fabric CA server configuration file.

To require that a client certificate also be specified, create the
account using the REQUIRE X509 option. Then the client must also specify
proper client key and certificate files; otherwise, the MySQL server
will reject the connection. To specify client key and certification files
for the Fabric CA server, set ``db.tls.certfiles``, ``db.tls.client.certfile``,
and the ``db.tls.client.keyfile`` configuration properties.

`Back to Top`_
=======
Certificate Authority (CA) Setup
================================

The *Certificate Authority* (CA) provides a number of certificate
services to users of a blockchain. More specifically, these services
relate to *user enrollment*, *transactions* invoked on the blockchain,
and *TLS*-secured connections between users or components of the
blockchain.

This guide builds on either the :doc:`fabric developer's
setup <../dev-setup/devenv>` or the prerequisites articulated in
the :doc:`fabric network setup <Network-setup>` guide. If you have not
already set up your environment with one of those guides, please do so
before continuing.

Enrollment Certificate Authority
--------------------------------

The *enrollment certificate authority* (ECA) allows new users to
register with the blockchain network and enables registered users to
request an *enrollment certificate pair*. One certificate is for data
signing, one is for data encryption. The public keys to be embedded in
the certificates have to be of type ECDSA, whereby the key for data
encryption is then converted by the user to be used in an
`ECIES <https://en.wikipedia.org/wiki/Integrated_Encryption_Scheme>`__
(Elliptic Curve Integrated Encryption System) fashion.

Transaction Certificate Authority
---------------------------------

Once a user is enrolled, he or she can also request *transaction
certificates* from the *transaction certificate authority* (TCA). These
certificates are to be used for deploying Chaincode and for invoking
Chaincode transactions on the blockchain. Although a single *transaction
certificate* can be used for multiple transactions, for privacy reasons
it is recommended that a new *transaction certificate* be used for each
transaction.

TLS Certificate Authority
-------------------------

In addition to *enrollment certificates* and *transaction certificates*,
users will need *TLS certificates* to secure their communication
channels. *TLS certificates* can be requested from the *TLS certificate
authority* (TLSCA).

Configuration
-------------

All CA services are provided by a single process, which can be
configured by setting parameters in the CA configuration file
``membersrvc.yaml``, which is located in the same directory as the CA
binary. More specifically, the following parameters can be set:

-  ``server.gomaxprocs``: limits the number of operating system threads
   used by the CA.
-  ``server.rootpath``: the root path of the directory where the CA
   stores its state.
-  ``server.cadir``: the name of the directory where the CA stores its
   state.
-  ``server.port``: the port at which all CA services listen
   (multiplexing of services over the same port is provided by
   `GRPC <http://www.grpc.io>`__).

Furthermore, logging levels can be enabled/disabled by adjusting the
following settings:

-  ``logging.trace`` (off by default, useful for debugging the code
   only)
-  ``logging.info``
-  ``logging.warning``
-  ``logging.error``
-  ``logging.panic``

Alternatively, these fields can be set via environment variables,
which---if set---have precedence over entries in the yaml file. The
corresponding environment variables are named as follows:

::

        MEMBERSRVC_CA_SERVER_GOMAXPROCS
        MEMBERSRVC_CA_SERVER_ROOTPATH
        MEMBERSRVC_CA_SERVER_CADIR
        MEMBERSRVC_CA_SERVER_PORT

In addition, the CA may be preloaded with registered users, where each
user's name, roles, and password are specified:

::

        eca:
            users:
                alice: 2 DRJ20pEql15a
                bob: 4 7avZQLwcUe9q

The role value is simply a bitmask of the following:

::

        CLIENT = 1;
        PEER = 2;
        VALIDATOR = 4;
        AUDITOR = 8;

For example, a peer that is also a validator would have a role value of
6.

When the CA is started for the first time, it will generate all of its
required state (e.g., internal databases, CA certificates, blockchain
keys, etc.) and writes this state to the directory given in its
configuration. The certificates for the CA services (i.e., for the ECA,
TCA, and TLSCA) are self-signed as the current default. If those
certificates shall be signed by some root CA, this can be done manually
by using the ``*.priv`` and ``*.pub`` private and public keys in the CA
state directory, and replacing the self-signed ``*.cert`` certificates
with root-signed ones. The next time the CA is launched, it will read
and use those root-signed certificates.

Operating the CA
----------------

You can either `build and run <#build-and-run>`__ the CA from source.
Or, you can use Docker Compose and work with the published images on
DockerHub, or some other Docker registry. Using Docker Compose is by far
the simplest approach.

Docker Compose
^^^^^^^^^^^^^^

Here's a sample docker-compose.yml for the CA.

::

    membersrvc:
      image: hyperledger/fabric-membersrvc
      command: membersrvc

The corresponding docker-compose.yml for running Docker on Mac or
Windows natively looks like this:

::

    membersrvc:
      image: hyperledger/fabric-membersrvc
      ports:
        - "7054:7054"
      command: membersrvc

If you are launching one or more ``peer`` nodes in the same
docker-compose.yml, then you will want to add a delay to the start of
the peer to allow sufficient time for the CA to start, before the peer
attempts to connect to it.

::

    membersrvc:
      image: hyperledger/fabric-membersrvc
      command: membersrvc
    vp0:
      image: hyperledger/fabric-peer
      environment:
        - CORE_PEER_ADDRESSAUTODETECT=true
        - CORE_VM_ENDPOINT=http://172.17.0.1:2375
        - CORE_LOGGING_LEVEL=DEBUG
        - CORE_PEER_ID=vp0
        - CORE_SECURITY_ENROLLID=test_vp0
        - CORE_SECURITY_ENROLLSECRET=MwYpmSRjupbT
      links:
        - membersrvc
      command: sh -c "sleep 5; peer node start"

The corresponding docker-compose.yml for running Docker on Mac or
Windows natively looks like this:

::

    membersrvc:
      image: hyperledger/fabric-membersrvc
      ports:
        - "7054:7054"
      command: membersrvc
    vp0:
      image: hyperledger/fabric-peer
      ports:
        - "7050:7050"
        - "7051:7051"
        - "7052:7052"
      environment:
        - CORE_PEER_ADDRESSAUTODETECT=true
        - CORE_VM_ENDPOINT=unix:///var/run/docker.sock
        - CORE_LOGGING_LEVEL=DEBUG
        - CORE_PEER_ID=vp0
        - CORE_SECURITY_ENROLLID=test_vp0
        - CORE_SECURITY_ENROLLSECRET=MwYpmSRjupbT
      links:
        - membersrvc
      command: sh -c "sleep 5; peer node start"

Build and Run
^^^^^^^^^^^^^

The CA can be built with the following command executed in the
``membersrvc`` directory:

::

    cd $GOPATH/src/github.com/hyperledger/fabric
    make membersrvc

The CA can be started with the following command:

::

    build/bin/membersrvc

**Note:** the CA must be started before any of the fabric peer nodes, to
allow the CA to have initialized before any peer nodes attempt to
connect to it.

The CA looks for an ``membersrvc.yaml`` configuration file in
$GOPATH/src/github.com/hyperledger/fabric/membersrvc. If the CA is
started for the first time, it creates all its required state (e.g.,
internal databases, CA certificates, blockchain keys, etc.) and writes
that state to the directory given in the CA configuration.

.. raw:: html

   <!-- This needs some serious attention

   If starting the peer with security/privacy enabled, environment variables for security, CA address and peer's ID and password must be included. Additionally, the fabric-membersrvc container must be started before the peer(s) are launched. Hence we will need to insert a delay in launching the peer command. Here's the docker-compose.yml for a single peer with membership services running in a **Vagrant** environment:

   ```
   vp0:
     image: hyperledger/fabric-peer
     environment:
     - CORE_PEER_ADDRESSAUTODETECT=true
     - CORE_VM_ENDPOINT=http://172.17.0.1:2375
     - CORE_LOGGING_LEVEL=DEBUG
     - CORE_PEER_ID=vp0
     - CORE_PEER_TLS_ENABLED=true
     - CORE_PEER_TLS_SERVERHOSTOVERRIDE=OBC
     - CORE_PEER_TLS_CERT_FILE=./bddtests/tlsca.cert
     - CORE_PEER_TLS_KEY_FILE=./bddtests/tlsca.priv
     command: sh -c "sleep 5; peer node start"

   membersrvc:
      image: hyperledger/fabric-membersrvc
      command: membersrvc
   ```

   ```
   docker run --rm -it -e CORE_VM_ENDPOINT=http://172.17.0.1:2375 -e CORE_PEER_ID=vp0 -e CORE_PEER_ADDRESSAUTODETECT=true -e CORE_SECURITY_ENABLED=true -e CORE_SECURITY_PRIVACY=true -e CORE_PEER_PKI_ECA_PADDR=172.17.0.1:7054 -e CORE_PEER_PKI_TCA_PADDR=172.17.0.1:7054 -e CORE_PEER_PKI_TLSCA_PADDR=172.17.0.1:7054 -e CORE_SECURITY_ENROLLID=vp0 -e CORE_SECURITY_ENROLLSECRET=vp0_secret  hyperledger/fabric-peer peer node start
   ```

   Additionally, the validating peer `enrollID` and `enrollSecret` (`vp0` and `vp0_secret`) has to be added to [membersrvc.yaml](https://github.com/hyperledger/fabric/blob/master/membersrvc/membersrvc.yaml).
   -->
>>>>>>> efef932... [FAB-2977] convert v0.6 .md to .rst
