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

//This pipeline builds all of istio then uploads the istio/operator, pilot, and proxyv2 images to dtr as well as saving them
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
		DISTROLESS_TAG = "${containerId}-distroless"
		BUILD_DATE = "${buildDate}"
		VERSION = "1.7.8"
		VARIANT = "cray1"
		IMAGE_TAG = getDockerImageTag(version: "${VERSION}-${VARIANT}", buildDate: "${BUILD_DATE}", gitTag: "${GIT_TAG}", gitBranch: "${GIT_BRANCH}")
		DISTROLESS_IMAGE_TAG = getDockerImageTag(version: "${VERSION}-${VARIANT}-distroless", buildDate: "${BUILD_DATE}", gitTag: "${GIT_TAG}", gitBranch: "${GIT_BRANCH}")
		IYUM_REPO_MAIN_BRANCH = "cray-master"
		PRODUCT = "csm"
		RELEASE_TAG = setReleaseTag()
		BUILD_WITH_CONTAINER = "1"
		DOCKER_BUILD_VARIANTS = "default distroless"
	}

	stages {
		stage('Make gen-charts') {
			steps {
				echo "Log Stash: istio Build Pipeline - Make gen-charts"
				echo "Make gen-charts"
				sh "make --debug=all gen-charts"
			}
		}
		stage('Make Build') {
			steps {
				echo "Log Stash: istio Build Pipeline - Make Build"
				echo "Make Build"
				sh "make --debug=all build"
			}
		}
		stage('Make Docker') {
			steps {
				echo "Log Stash: istio Build Pipeline - Make Build"
				echo "Make Docker"
				sh "make --debug=all docker"
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
									imageVersioned: "istio/pilot:${TAG}"
									)
				publishDockerUtilityImage( imageTag: env.IMAGE_TAG,
									imageName: "operator",
									repository: "cray",
									imageVersioned: "istio/operator:${TAG}"
									)
				publishDockerUtilityImage( imageTag: env.IMAGE_TAG,
									imageName: "proxyv2",
									repository: "cray",
									imageVersioned: "istio/proxyv2:${TAG}"
									)
				publishDockerUtilityImage( imageTag: env.DISTROLESS_IMAGE_TAG,
									imageName: "pilot",
									repository: "cray",
									imageVersioned: "istio/pilot:${DISTROLESS_TAG}"
									)
				publishDockerUtilityImage( imageTag: env.DISTROLESS_IMAGE_TAG,
									imageName: "operator",
									repository: "cray",
									imageVersioned: "istio/operator:${DISTROLESS_TAG}"
									)
				publishDockerUtilityImage( imageTag: env.DISTROLESS_IMAGE_TAG,
									imageName: "proxyv2",
									repository: "cray",
									imageVersioned: "istio/proxyv2:${DISTROLESS_TAG}"
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
					docker rmi istio/operator:${TAG}||true
					docker rmi istio/istioctl:${TAG}||true
					docker rmi istio/node-agent-k8s:${TAG}||true
					docker rmi istio/kubectl:${TAG}||true
					docker rmi istio/sidecar_injector:${TAG}||true
					docker rmi istio/galley:${TAG}||true
					docker rmi istio/citadel:${TAG}||true
					docker rmi istio/mixer_codegen:${TAG}||true
					docker rmi istio/mixer:${TAG}||true
					docker rmi istio/test_policybackend:${TAG}||true
					docker rmi istio/app_sidecar:${TAG}||true
					docker rmi istio/app:${TAG}||true
					docker rmi istio/proxyv2:${TAG}||true
					docker rmi istio/proxytproxy:${TAG}||true
					docker rmi istio/pilot:${TAG}||true

					docker rmi istio/operator:${DISTROLESS_TAG}||true
					docker rmi istio/istioctl:${DISTROLESS_TAG}||true
					docker rmi istio/node-agent-k8s:${DISTROLESS_TAG}||true
					docker rmi istio/kubectl:${DISTROLESS_TAG}||true
					docker rmi istio/sidecar_injector:${DISTROLESS_TAG}||true
					docker rmi istio/galley:${DISTROLESS_TAG}||true
					docker rmi istio/citadel:${DISTROLESS_TAG}||true
					docker rmi istio/mixer_codegen:${DISTROLESS_TAG}||true
					docker rmi istio/mixer:${DISTROLESS_TAG}||true
					docker rmi istio/test_policybackend:${DISTROLESS_TAG}||true
					docker rmi istio/app_sidecar:${DISTROLESS_TAG}||true
					docker rmi istio/app:${DISTROLESS_TAG}||true
					docker rmi istio/proxyv2:${DISTROLESS_TAG}||true
					docker rmi istio/proxytproxy:${DISTROLESS_TAG}||true
					docker rmi istio/pilot:${DISTROLESS_TAG}||true
				"""
			}
		}

		stage('Push to github') {
		    when { allOf {
		        // Regex can be changed, as exampled above, to only run specific branches instead of all
		        expression { BRANCH_NAME ==~ /(release\/.*|cray-master)/ }
		    }}
		    steps {
		        script {
		            pushToGithub(
		                githubRepo: "Cray-HPE/istio",
		                pemSecretId: "githubapp-stash-sync",
		                githubAppId: "91129",
		                githubAppInstallationId: "13313749"
		            )
		        }
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

