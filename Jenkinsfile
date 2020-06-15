@Library('dst-shared@master') _

// Defined variables
def varDate = new Date()
def buildDate = varDate.format("yyyyMMddHHmmss") 
def containerId = UUID.randomUUID().toString()

if ("${BRANCH_NAME}" ==~ /PR-.*/) {
	buildAgent = "dstprbuild"
} else if (setReleaseTag()?.trim()) {
	buildAgent = "dsttagbuild"
} else {
	buildAgent = "dstbranchbuild"
}

//This pipeline builds all of istio with a fixed section for pilot, then uploads the istio/pilot image to dtr as well as saving it 
pipeline {
    agent { node { label buildAgent } }

	// Configuration options applicable to the entire job
	options {
		// This build should not take long, fail the build if it appears stuck
		timeout(time: 180, unit: 'MINUTES')

		// Don't fill up the build server with unnecessary cruft
		buildDiscarder(logRotator(numToKeepStr: '10'))

		// Disable concurrent builds to minimize build collisions
		disableConcurrentBuilds()

		// Add timestamps and color to console output, cuz pretty
		timestamps()
	}

	environment {
		GIT_TAG = sh(returnStdout: true, script: "git rev-parse --short HEAD").trim()
		TAG = "${containerId}"
		BUILD_DATE = "${buildDate}"
		VERSION = "1.5.4"
		IMAGE_TAG = getDockerImageTag(version: "${VERSION}", buildDate: "${BUILD_DATE}", gitTag: "${GIT_TAG}", gitBranch: "${GIT_BRANCH}")
		IYUM_REPO_MAIN_BRANCH = "cray-master"
		PRODUCT = "shasta-standard,shasta-premium"
		RELEASE_TAG = setReleaseTag()
		BUILD_WITH_CONTAINER = "1"
	}

	stages {
		stage('Make Build') {
			steps {
				echo "Log Stash: istio Build Pipeline - Make Build"
				echo "Make Build"
				sh "make -d build"
			}
		}
		stage('Make Docker') {
			steps {
				echo "Log Stash: istio Build Pipeline - Make Build"
				echo "Make Docker"
				sh "make -d docker"
			}
		}
		stage('Tag and Save image') {
			steps {
				dockerRetagAndSave(imageReference: "istio/pilot:{$TAG}",
					imageRepo: "dtr.dev.cray.com",
					imageName: "pilot",
					imageTag: "${IMAGE_TAG}",
					repository: "cray")
			}
		}

		// Publish
		stage('Publish') {
			environment {
				TARGET_OS = "noos"
				TARGET_ARCH = "noarch"
			}
			steps {
				echo "Log Stash: istio Build Pipeline - Publish"
				publishDockerUtilityImage( imageTag: env.IMAGE_TAG,
									imageName: "pilot",
									repository: "cray",
									imageVersioned: "istio/pilot:{$TAG}"
									)
				findAndTransferArtifacts()
			}
		}

		// Once the image has been pushed, lets untag it so it's not sitting around and consuming
		// valuable hardware space.
		stage('Docker cleanup') {
			steps {
				// docker rmi will remove the tagged tag without removing the original image
				sh """
					docker rmi istio/operator:{$TAG}
					docker rmi istio/istioctl:{$TAG}
					docker rmi istio/node-agent-k8s:{$TAG}
					docker rmi istio/kubectl:{$TAG}
					docker rmi istio/sidecar_injector:{$TAG}
					docker rmi istio/galley:{$TAG}
					docker rmi istio/citadel:{$TAG}
					docker rmi istio/mixer_codegen:{$TAG}
					docker rmi istio/mixer:{$TAG}
					docker rmi istio/test_policybackend:{$TAG}
					docker rmi istio/app_sidecar:{$TAG}
					docker rmi istio/app:{$TAG}
					docker rmi istio/proxyv2:{$TAG}
					docker rmi istio/proxytproxy:{$TAG}
					docker rmi istio/pilot:{$TAG}
				"""
			}
		}
	}

	post('Post-build steps') {
		always {
			script {
				currentBuild.result = currentBuild.result == null ? "SUCCESS" : currentBuild.result
			}
			logstashSend failBuild: false, maxLines: 3000
		}
	}
}

