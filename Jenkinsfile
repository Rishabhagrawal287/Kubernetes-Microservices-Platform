// Reference Jenkinsfile — demonstrates the same build/test/scan/push/deploy
// stages as .github/workflows/ci.yml, in Jenkins declarative pipeline syntax.
//
// This project's actual CI runs through GitHub Actions (see README) — Jenkins
// isn't running anywhere for this project. This file exists because the
// original spec asked for a Jenkins pipeline, and because being able to
// write one is worth demonstrating even when it isn't what you run day to
// day. A few things below (the "ghcr-credentials" ID, the deploy target)
// are placeholders that assume a real Jenkins instance with its own
// configured credentials store — they won't resolve to anything as-is.

pipeline {
    agent any

    options {
        buildDiscarder(logRotator(numToKeepStr: '20'))
        timestamps()
    }

    environment {
        REGISTRY      = 'ghcr.io'
        IMAGE_OWNER   = 'rishabhagrawal287' // must be lowercase — Docker requirement
        GIT_SHORT_SHA = "${env.GIT_COMMIT ? env.GIT_COMMIT.take(7) : 'local'}"
    }

    stages {

        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Build') {
            parallel {
                stage('user-service') {
                    steps {
                        sh 'docker build -t user-service:${GIT_SHORT_SHA} ./services/user-service'
                    }
                }
                stage('order-service') {
                    steps {
                        sh 'docker build -t order-service:${GIT_SHORT_SHA} ./services/order-service'
                    }
                }
                stage('product-service') {
                    steps {
                        sh 'docker build -t product-service:${GIT_SHORT_SHA} ./services/product-service'
                    }
                }
            }
        }

        stage('Unit Tests') {
            // Placeholders — this project doesn't have unit tests yet (Phase 2
            // focused on getting the real logic and event flow working first).
            // Add real ones here as the codebase grows.
            parallel {
                stage('user-service') {
                    steps {
                        sh '''
                            cd services/user-service
                            npm install
                            echo "no unit tests yet — placeholder stage"
                        '''
                    }
                }
                stage('order-service') {
                    steps {
                        sh '''
                            cd services/order-service
                            pip install -r requirements.txt --break-system-packages
                            echo "no unit tests yet — placeholder stage"
                        '''
                    }
                }
                stage('product-service') {
                    steps {
                        sh '''
                            cd services/product-service
                            go vet ./...
                        '''
                    }
                }
            }
        }

        stage('Security Scan (Trivy)') {
            steps {
                sh '''
                    for svc in user-service order-service product-service; do
                        trivy image --severity CRITICAL,HIGH --exit-code 0 ${svc}:${GIT_SHORT_SHA}
                    done
                '''
            }
        }

        stage('Integration Test (Kind)') {
            steps {
                sh '''
                    kind create cluster --name jenkins-ci --config infra/kind-config.yaml || true

                    for svc in user-service order-service product-service; do
                        kind load docker-image ${svc}:${GIT_SHORT_SHA} --name jenkins-ci
                    done

                    kubectl apply -f k8s/infra/namespace.yaml
                    kubectl apply -f k8s/infra/secrets.yaml
                    kubectl apply -f k8s/infra/mongo.yaml
                    kubectl apply -f k8s/infra/postgres.yaml
                    kubectl apply -f k8s/infra/redis.yaml
                    kubectl apply -f k8s/infra/rabbitmq.yaml
                    kubectl wait --for=condition=ready pod -l app=rabbitmq -n microservices --timeout=180s

                    helm install user-service ./helm/user-service -n microservices -f ./helm/user-service/values-dev.yaml --set image.tag=${GIT_SHORT_SHA}
                    helm install order-service ./helm/order-service -n microservices -f ./helm/order-service/values-dev.yaml --set image.tag=${GIT_SHORT_SHA}
                    helm install product-service ./helm/product-service -n microservices -f ./helm/product-service/values-dev.yaml --set image.tag=${GIT_SHORT_SHA}

                    kubectl wait --for=condition=available deployment --all -n microservices --timeout=120s

                    chmod +x ./scripts/integration-test.sh
                    ./scripts/integration-test.sh
                '''
            }
        }

        stage('Push Images') {
            when {
                branch 'main'
            }
            steps {
                withCredentials([usernamePassword(credentialsId: 'ghcr-credentials', usernameVariable: 'GHCR_USER', passwordVariable: 'GHCR_TOKEN')]) {
                    sh '''
                        echo "$GHCR_TOKEN" | docker login ${REGISTRY} -u "$GHCR_USER" --password-stdin
                        for svc in user-service order-service product-service; do
                            IMAGE=${REGISTRY}/${IMAGE_OWNER}/microservices-platform-${svc}
                            docker tag ${svc}:${GIT_SHORT_SHA} ${IMAGE}:latest
                            docker tag ${svc}:${GIT_SHORT_SHA} ${IMAGE}:${GIT_SHORT_SHA}
                            docker push ${IMAGE}:latest
                            docker push ${IMAGE}:${GIT_SHORT_SHA}
                        done
                    '''
                }
            }
        }

        stage('Deploy (Blue/Green)') {
            when {
                branch 'main'
            }
            steps {
                sh '''
                    # Blue/Green via Helm: install the new version under a
                    # "-green" release name, verify it, then cut over by
                    # upgrading the real release once it's confirmed healthy.
                    # This mirrors the Blue/Green vs Canary discussion from
                    # earlier in this project. Left as a reference sketch —
                    # actually running this needs a real target cluster and
                    # credentials, which this repo intentionally doesn't have.
                    echo "Deploy stage placeholder — see comment above."
                '''
            }
        }

        stage('Rollback (on failure)') {
            when {
                expression { currentBuild.result == 'FAILURE' }
            }
            steps {
                sh '''
                    for svc in user-service order-service product-service; do
                        helm rollback ${svc} -n microservices || true
                    done
                '''
            }
        }
    }

    post {
        always {
            sh 'kind delete cluster --name jenkins-ci || true'
        }
        failure {
            echo 'Build failed — see stage logs above for details.'
        }
    }
}
