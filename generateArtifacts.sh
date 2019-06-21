#!/bin/bash +x
#
# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#


#set -e

CHANNEL_NAME1=$1
: ${CHANNEL_NAME1:="channel1"}
echo $CHANNEL_NAME1
CHANNEL_NAME2=channel2

export FABRIC_ROOT=$PWD/../..
export FABRIC_CFG_PATH=$PWD
echo

OS_ARCH=$(echo "$(uname -s|tr '[:upper:]' '[:lower:]'|sed 's/mingw64_nt.*/windows/')-$(uname -m | sed 's/x86_64/amd64/g')" | awk '{print tolower($0)}')

## Using docker-compose template replace private key file names with constants
function replacePrivateKey () {
	ARCH=`uname -s | grep Darwin`
	if [ "$ARCH" == "Darwin" ]; then
		OPTS="-it"
	else
		OPTS="-i"
	fi

	cp docker-compose-e2e-template.yaml docker-compose-e2e.yaml

        CURRENT_DIR=$PWD
        cd crypto-config/peerOrganizations/org1.example.com/ca/
        PRIV_KEY=$(ls *_sk)
        cd $CURRENT_DIR
        sed $OPTS "s/CA1_PRIVATE_KEY/${PRIV_KEY}/g" docker-compose-e2e.yaml
        cd crypto-config/peerOrganizations/org2.example.com/ca/
        PRIV_KEY=$(ls *_sk)
        cd $CURRENT_DIR
        sed $OPTS "s/CA2_PRIVATE_KEY/${PRIV_KEY}/g" docker-compose-e2e.yaml
		cd crypto-config/peerOrganizations/org3.example.com/ca/
        PRIV_KEY=$(ls *_sk)
        cd $CURRENT_DIR
        sed $OPTS "s/CA3_PRIVATE_KEY/${PRIV_KEY}/g" docker-compose-e2e.yaml
}

## Generates Org certs using cryptogen tool
function generateCerts (){
	CRYPTOGEN=$FABRIC_ROOT/release/$OS_ARCH/bin/cryptogen

	if [ -f "$CRYPTOGEN" ]; then
            echo "Using cryptogen -> $CRYPTOGEN"
	else
	    echo "Building cryptogen"
	    make -C $FABRIC_ROOT release
	fi

	echo
	echo "##########################################################"
	echo "##### Generate certificates using cryptogen tool #########"
	echo "##########################################################"
	$CRYPTOGEN generate --config=./crypto-config.yaml
	echo
}

function generateIdemixMaterial (){
	IDEMIXGEN=$FABRIC_ROOT/release/$OS_ARCH/bin/idemixgen
	CURDIR=`pwd`
	IDEMIXMATDIR=$CURDIR/crypto-config/idemix

	if [ -f "$IDEMIXGEN" ]; then
            echo "Using idemixgen -> $IDEMIXGEN"
	else
	    echo "Building idemixgen"
	    make -C $FABRIC_ROOT release
	fi

	echo
	echo "####################################################################"
	echo "##### Generate idemix crypto material using idemixgen tool #########"
	echo "####################################################################"

	mkdir -p $IDEMIXMATDIR
	cd $IDEMIXMATDIR

	# Generate the idemix issuer keys
	$IDEMIXGEN ca-keygen

	# Generate the idemix signer keys
	$IDEMIXGEN signerconfig -u OU1 -e OU1 -r 1

	cd $CURDIR
}

## Generate orderer genesis block , channel configuration transaction and anchor peer update transactions
function generateChannelArtifacts() {

	CONFIGTXGEN=$FABRIC_ROOT/release/$OS_ARCH/bin/configtxgen
	if [ -f "$CONFIGTXGEN" ]; then
            echo "Using configtxgen -> $CONFIGTXGEN"
	else
	    echo "Building configtxgen"
	    make -C $FABRIC_ROOT release
	fi

	echo "##########################################################"
	echo "#########  Generating Orderer Genesis block ##############"
	echo "##########################################################"
	# Note: For some unknown reason (at least for now) the block file can't be
	# named orderer.genesis.block or the orderer will fail to launch!
	$CONFIGTXGEN -profile TwoOrgsOrdererGenesis -channelID e2e-orderer-syschan -outputBlock ./channel-artifacts/genesis.block

	echo
	echo "#################################################################"
	echo "### Generating channel11 configuration transaction 'channel1.tx' ###"
	echo "#################################################################"
	$CONFIGTXGEN -profile TwoOrgsChannel1 -outputCreateChannelTx ./channel-artifacts/channel1.tx -channelID $CHANNEL_NAME1

	echo
	echo "#################################################################"
	echo "#######    Generating anchor peer update for Org1MSP   ##########"
	echo "#################################################################"
	$CONFIGTXGEN -profile TwoOrgsChannel1 -outputAnchorPeersUpdate ./channel-artifacts/Org1MSPanchors1.tx -channelID $CHANNEL_NAME1 -asOrg Org1MSP

	echo
	echo "#################################################################"
	echo "#######    Generating anchor peer update for Org3MSP   ##########"
	echo "#################################################################"
	$CONFIGTXGEN -profile TwoOrgsChannel1 -outputAnchorPeersUpdate ./channel-artifacts/Org3MSPanchors1.tx -channelID $CHANNEL_NAME1 -asOrg Org3MSP
	echo

	echo
	echo "#################################################################"
	echo "### Generating channel2 configuration transaction 'channel2.tx' ###"
	echo "#################################################################"
	$CONFIGTXGEN -profile TwoOrgsChannel2 -outputCreateChannelTx ./channel-artifacts/channel2.tx -channelID $CHANNEL_NAME2

	echo
	echo "#################################################################"
	echo "#######    Generating anchor peer update for Org1MSP   ##########"
	echo "#################################################################"
	$CONFIGTXGEN -profile TwoOrgsChannel2 -outputAnchorPeersUpdate ./channel-artifacts/Org2MSPanchors2.tx -channelID $CHANNEL_NAME2 -asOrg Org2MSP

	echo
	echo "#################################################################"
	echo "#######    Generating anchor peer update for Org3MSP   ##########"
	echo "#################################################################"
	$CONFIGTXGEN -profile TwoOrgsChannel2 -outputAnchorPeersUpdate ./channel-artifacts/Org3MSPanchors2.tx -channelID $CHANNEL_NAME2  -asOrg Org3MSP
	echo
}

generateCerts
generateIdemixMaterial
replacePrivateKey
generateChannelArtifacts