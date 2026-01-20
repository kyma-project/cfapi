package btp_test

import (
	"context"
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/brokerapi/v13/domain"
	btpv1 "github.com/SAP/sap-btp-service-operator/api/v1"
	"github.com/SAP/sap-btp-service-operator/client/sm/types"
	"github.com/google/uuid"
	"github.com/kyma-project/cfapi/components/btp-service-broker/btp"
	"github.com/kyma-project/cfapi/components/btp-service-broker/btp/fake"
	"github.com/kyma-project/cfapi/components/btp-service-broker/tools/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	True  bool = true
	False bool = false
)

var _ = Describe("BTPBroker", func() {
	var (
		smClient           *fake.SMClient
		credentialsDecoder *fake.CredentialsDecoder
		broker             *btp.BTPBroker
	)

	BeforeEach(func() {
		ctx = context.Background()
		smClient = new(fake.SMClient)

		servicePlan := types.ServicePlan{
			ID:          "plan-id",
			Name:        "plan-name",
			Description: "plan-description",
			Free:        true,
			Bindable:    true,
			Metadata: json.RawMessage([]byte(`{
				"displayName": "plan-display-name",
				"bullets": ["bullet1", "bullet2"],
				"costs": [{"unit": "instance", "amount": {"USD": 0}}]
			}`)),
			ServiceOfferingID: "offering-id",
		}

		smClient.ListOfferingsReturns(&types.ServiceOfferings{
			ServiceOfferings: []types.ServiceOffering{{
				ID:            "offering-id",
				Name:          "offering-name",
				Description:   "offering-description",
				Bindable:      true,
				Tags:          json.RawMessage(`["tag1", "tag2"]`),
				PlanUpdatable: true,
				Plans:         []types.ServicePlan{servicePlan},
				Requires:      json.RawMessage{},
				Metadata: json.RawMessage([]byte(`{
					"displayName": "offering-display-name",
					"imageUrl": "https://example.com/image.png",
					"longDescription": "offering-long-description",
					"providerDisplayName": "provider-display-name",
					"documentationUrl": "https://example.com/docs",
					"supportUrl": "https://example.com/support",
					"shareable": true
				}`)),
			}},
		}, nil)

		smClient.ListPlansReturns(&types.ServicePlans{
			ServicePlans: []types.ServicePlan{servicePlan},
		}, nil)

		credentialsDecoder = new(fake.CredentialsDecoder)

		broker = btp.NewBroker(k8sClient, smClient, resourceNamespace, credentialsDecoder)
	})

	Describe("Services", func() {
		var (
			services []domain.Service
			err      error
		)

		JustBeforeEach(func() {
			services, err = broker.Services(ctx)
		})

		It("calls the SM client to list offerings and plans", func() {
			Expect(smClient.ListOfferingsCallCount()).To(Equal(1))
			Expect(smClient.ListPlansCallCount()).To(Equal(1))
		})

		It("returns the list of services", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(services).To(ConsistOf(domain.Service{
				ID:            "offering-id",
				Name:          "offering-name",
				Description:   "offering-description",
				Bindable:      true,
				Tags:          []string{"tag1", "tag2"},
				PlanUpdatable: true,
				Plans: []domain.ServicePlan{{
					ID:          "plan-id",
					Name:        "plan-name",
					Description: "plan-description",
					Free:        &True,
					Bindable:    &True,
					Metadata: &domain.ServicePlanMetadata{
						DisplayName: "plan-display-name",
						Bullets:     []string{"bullet1", "bullet2"},
						Costs: []domain.ServicePlanCost{{
							Amount: map[string]float64{
								"USD": 0,
							},
							Unit: "instance",
						}},
					},
				}},
				Metadata: &domain.ServiceMetadata{
					DisplayName:         "offering-display-name",
					ImageUrl:            "https://example.com/image.png",
					LongDescription:     "offering-long-description",
					ProviderDisplayName: "provider-display-name",
					DocumentationUrl:    "https://example.com/docs",
					SupportUrl:          "https://example.com/support",
					Shareable:           &True,
				},
			}))
		})

		When("there are many offerings and plans", func() {
			BeforeEach(func() {
				smClient.ListOfferingsReturns(&types.ServiceOfferings{
					ServiceOfferings: []types.ServiceOffering{
						{ID: "offering-1"},
						{ID: "offering-2"},
						{ID: "offering-no-plan"},
					},
				}, nil)
				smClient.ListPlansReturns(&types.ServicePlans{
					ServicePlans: []types.ServicePlan{
						{ID: "plan-a", ServiceOfferingID: "offering-1"},
						{ID: "plan-b", ServiceOfferingID: "offering-2"},
						{ID: "plan-c", ServiceOfferingID: "offering-2"},
						{ID: "plan-no-offering", ServiceOfferingID: ""},
					},
				}, nil)
			})

			It("returns maps offerings and plans", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(services).To(ConsistOf(
					domain.Service{
						ID: "offering-1",
						Plans: []domain.ServicePlan{
							{ID: "plan-a", Free: &False, Bindable: &False},
						},
					},
					domain.Service{
						ID: "offering-2",
						Plans: []domain.ServicePlan{
							{ID: "plan-b", Free: &False, Bindable: &False},
							{ID: "plan-c", Free: &False, Bindable: &False},
						},
					},
					domain.Service{
						ID: "offering-no-plan",
					},
				))
			})
		})

		When("the offering tags are invalid", func() {
			BeforeEach(func() {
				smClient.ListOfferingsReturns(&types.ServiceOfferings{
					ServiceOfferings: []types.ServiceOffering{
						{ID: "offering-1", Tags: []byte("invalid")},
					},
				}, nil)
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to unmarshal service offering tags")))
			})
		})

		When("the offering metadata is invalid", func() {
			BeforeEach(func() {
				smClient.ListOfferingsReturns(&types.ServiceOfferings{
					ServiceOfferings: []types.ServiceOffering{
						{ID: "offering-1", Metadata: []byte("invalid")},
					},
				}, nil)
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to unmarshal service offering metadata")))
			})
		})

		When("the offering required permissions are invalid", func() {
			BeforeEach(func() {
				smClient.ListOfferingsReturns(&types.ServiceOfferings{
					ServiceOfferings: []types.ServiceOffering{
						{ID: "offering-1", Requires: []byte("invalid")},
					},
				}, nil)
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to unmarshal required permissions")))
			})
		})

		When("the SM client fails to list offerings", func() {
			BeforeEach(func() {
				smClient.ListOfferingsReturns(nil, errors.New("failed to list offerings"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to list offerings")))
			})
		})

		When("the SM client fails to list plans", func() {
			BeforeEach(func() {
				smClient.ListPlansReturns(nil, errors.New("failed to list plans"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to list plans")))
			})
		})

		When("the raw json service fields are empty", func() {
			BeforeEach(func() {
				smClient.ListOfferingsReturns(&types.ServiceOfferings{
					ServiceOfferings: []types.ServiceOffering{{
						ID: "offering-id",
					}},
				}, nil)
				smClient.ListPlansReturns(&types.ServicePlans{
					ServicePlans: []types.ServicePlan{{
						ID: "plan-id",
					}},
				}, nil)
			})

			It("returns services with empty metadata", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(services).To(ConsistOf(domain.Service{
					ID: "offering-id",
				}))
			})
		})
	})

	Describe("Provision", func() {
		var (
			provisionDetails domain.ProvisionDetails
			instanceID       string
			service          domain.ProvisionedServiceSpec
			err              error
		)

		BeforeEach(func() {
			instanceID = uuid.NewString()
			provisionDetails = domain.ProvisionDetails{
				ServiceID: "offering-id",
				PlanID:    "plan-id",
			}
		})

		JustBeforeEach(func() {
			service, err = broker.Provision(ctx, instanceID, provisionDetails, true)
		})

		It("creates a btp service instance", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(service).To(Equal(domain.ProvisionedServiceSpec{
				IsAsync:       true,
				OperationData: "provision-" + instanceID,
			}))

			btpServiceInstance := &btpv1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instanceID,
					Namespace: resourceNamespace,
				},
			}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(btpServiceInstance), btpServiceInstance)).To(Succeed())
			Expect(btpServiceInstance.Spec.ServiceOfferingName).To(Equal("offering-name"))
		})

		When("offering id does not exist", func() {
			BeforeEach(func() {
				provisionDetails.ServiceID = "does-not-exist"
			})

			It("returns error", func() {
				Expect(err).To(MatchError(ContainSubstring("not found")))
			})
		})

		When("plan id does not exist", func() {
			BeforeEach(func() {
				provisionDetails.PlanID = "does-not-exist"
			})

			It("returns error", func() {
				Expect(err).To(MatchError(ContainSubstring("not found")))
			})
		})
	})

	Describe("Deprovision", func() {
		var (
			btpServiceInstance *btpv1.ServiceInstance
			instanceID         string
			operation          domain.DeprovisionServiceSpec
		)
		BeforeEach(func() {
			instanceID = uuid.NewString()
			btpServiceInstance = &btpv1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instanceID,
					Namespace: resourceNamespace,
				},
				Spec: btpv1.ServiceInstanceSpec{
					ServiceOfferingName: "offering-name",
					ServicePlanName:     "plan-name	",
					ServicePlanID:       "plan-id	",
				},
			}
			Expect(k8sClient.Create(ctx, btpServiceInstance)).To(Succeed())
		})

		JustBeforeEach(func() {
			var err error
			operation, err = broker.Deprovision(ctx, instanceID, domain.DeprovisionDetails{}, false)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the btp service instance", func() {
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(btpServiceInstance), btpServiceInstance)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
		})

		It("returns async deprovision operation", func() {
			Expect(operation).To(Equal(domain.DeprovisionServiceSpec{
				IsAsync:       true,
				OperationData: "deprovision-" + instanceID,
			}))
		})

		When("the instance does not exist", func() {
			BeforeEach(func() {
				instanceID = uuid.NewString()
			})

			It("still succeeds and returns async deprovision operation", func() {
				Expect(operation).To(Equal(domain.DeprovisionServiceSpec{
					IsAsync:       true,
					OperationData: "deprovision-" + instanceID,
				}))
			})
		})
	})

	Describe("LastOperation", func() {
		var (
			instanceID         string
			details            domain.PollDetails
			lastOp             domain.LastOperation
			err                error
			btpServiceInstance *btpv1.ServiceInstance
		)

		BeforeEach(func() {
			instanceID = uuid.NewString()

			btpServiceInstance = &btpv1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instanceID,
					Namespace: resourceNamespace,
				},
				Spec: btpv1.ServiceInstanceSpec{
					ServiceOfferingName: "offering-name",
					ServicePlanName:     "plan-name",
					ServicePlanID:       "plan-id",
				},
			}
			details = domain.PollDetails{}

			Expect(k8sClient.Create(ctx, btpServiceInstance)).To(Succeed())
		})

		JustBeforeEach(func() {
			lastOp, err = broker.LastOperation(ctx, instanceID, details)
		})

		Describe("provision last operation", func() {
			BeforeEach(func() {
				Expect(k8s.Patch(ctx, k8sClient, btpServiceInstance, func() {
					btpServiceInstance.Status = btpv1.ServiceInstanceStatus{
						Ready:         "True",
						OperationType: "create",
						Conditions: []metav1.Condition{
							{
								Type:               "Succeeded",
								Status:             metav1.ConditionFalse,
								Reason:             "Created",
								ObservedGeneration: btpServiceInstance.Generation,
								LastTransitionTime: metav1.Now(),
							},
						},
					}
				})).To(Succeed())
				details.OperationData = "provision-" + instanceID
			})

			It("returns the last operation state", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(lastOp.State).To(Equal(domain.InProgress))
			})

			When("the provision has succeeded", func() {
				BeforeEach(func() {
					Expect(k8s.Patch(ctx, k8sClient, btpServiceInstance, func() {
						btpServiceInstance.Status = btpv1.ServiceInstanceStatus{
							Ready: "True",
							Conditions: []metav1.Condition{
								{
									Type:               "Succeeded",
									Status:             metav1.ConditionTrue,
									Reason:             "Created",
									ObservedGeneration: btpServiceInstance.Generation,
									Message:            "Service instance created successfully",
									LastTransitionTime: metav1.Now(),
								},
							},
						}
					})).To(Succeed())
				})

				It("returns provision succeess last operation response", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(lastOp.State).To(Equal(domain.Succeeded))
				})
			})

			When("instance creation is failed", func() {
				BeforeEach(func() {
					Expect(k8s.Patch(ctx, k8sClient, btpServiceInstance, func() {
						btpServiceInstance.Status = btpv1.ServiceInstanceStatus{
							Ready: "True",
							Conditions: []metav1.Condition{
								{
									Type:               "Succeeded",
									Status:             metav1.ConditionFalse,
									Reason:             "CreateFailed",
									ObservedGeneration: btpServiceInstance.Generation,
									LastTransitionTime: metav1.Now(),
								},
								{
									Type:               "Failed",
									Status:             metav1.ConditionTrue,
									Reason:             "CreateFailed",
									ObservedGeneration: btpServiceInstance.Generation,
									Message:            "Create failed",
									LastTransitionTime: metav1.Now(),
								},
							},
						}
					})).To(Succeed())
				})

				It("returns the last operation state as failed", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(lastOp).To(Equal(domain.LastOperation{
						State:       domain.Failed,
						Description: "Create failed",
					}))
				})
			})

			When("instance status is not the latest", func() {
				BeforeEach(func() {
					Expect(k8s.Patch(ctx, k8sClient, btpServiceInstance, func() {
						btpServiceInstance.Status = btpv1.ServiceInstanceStatus{
							Ready:         "True",
							OperationType: "create",
							Conditions: []metav1.Condition{
								{
									Type:               "Succeeded",
									Status:             metav1.ConditionTrue,
									Reason:             "Created",
									ObservedGeneration: btpServiceInstance.Generation - 1,
									LastTransitionTime: metav1.Now(),
								},
							},
						}
					})).To(Succeed())
				})

				It("returns the last operation state as in progress", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(lastOp.State).To(Equal(domain.InProgress))
				})

				When("operation type is not create", func() {
					BeforeEach(func() {
						Expect(k8s.Patch(ctx, k8sClient, btpServiceInstance, func() {
							btpServiceInstance.Status.OperationType = "update"
						})).To(Succeed())
					})

					It("returns empty last operation", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(lastOp).To(Equal(domain.LastOperation{}))
					})
				})
			})

			When("the instance does not exist", func() {
				BeforeEach(func() {
					instanceID = uuid.NewString()
				})

				It("returns a not found err", func() {
					Expect(err).To(MatchError(ContainSubstring("instance does not exist")))
				})
			})
		})

		Describe("deprovision last operation", func() {
			BeforeEach(func() {
				details.OperationData = "deprovision-" + instanceID

				Expect(k8s.Patch(ctx, k8sClient, btpServiceInstance, func() {
					btpServiceInstance.Status = btpv1.ServiceInstanceStatus{
						Ready:         "True",
						OperationType: "delete",
						Conditions: []metav1.Condition{
							{
								Type:               "Succeeded",
								Status:             metav1.ConditionFalse,
								Reason:             "Deleted",
								ObservedGeneration: btpServiceInstance.Generation,
								LastTransitionTime: metav1.Now(),
							},
						},
					}
				})).To(Succeed())
			})

			It("returns deprovision in progress last operation response", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(lastOp.State).To(Equal(domain.InProgress))
			})

			When("the instance is gone", func() {
				BeforeEach(func() {
					instanceID = uuid.NewString()
				})

				It("returns a succeeded response", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(lastOp.State).To(Equal(domain.Succeeded))
				})
			})

			When("deprovision failed", func() {
				BeforeEach(func() {
					Expect(k8s.Patch(ctx, k8sClient, btpServiceInstance, func() {
						btpServiceInstance.Status = btpv1.ServiceInstanceStatus{
							Ready:         "True",
							OperationType: "delete",
							Conditions: []metav1.Condition{
								{
									Type:               "Failed",
									Status:             metav1.ConditionTrue,
									Reason:             "Failed",
									ObservedGeneration: btpServiceInstance.Generation,
									Message:            "Deprovision failed",
									LastTransitionTime: metav1.Now(),
								},
							},
						}
					})).To(Succeed())
				})

				It("returns a failed response", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(lastOp).To(Equal(domain.LastOperation{
						State:       domain.Failed,
						Description: "Deprovision failed",
					}))
				})

				When("instance status is not the latest", func() {
					BeforeEach(func() {
						Expect(k8s.Patch(ctx, k8sClient, btpServiceInstance, func() {
							btpServiceInstance.Status = btpv1.ServiceInstanceStatus{
								Ready:         "True",
								OperationType: "delete",
								Conditions: []metav1.Condition{
									{
										Type:               "Succeeded",
										Status:             metav1.ConditionTrue,
										Reason:             "Deleted",
										ObservedGeneration: btpServiceInstance.Generation - 1,
										LastTransitionTime: metav1.Now(),
									},
								},
							}
						})).To(Succeed())
					})

					It("returns the last operation state as in progress", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(lastOp.State).To(Equal(domain.InProgress))
					})

					When("operation type is not delete", func() {
						BeforeEach(func() {
							Expect(k8s.Patch(ctx, k8sClient, btpServiceInstance, func() {
								btpServiceInstance.Status.OperationType = "update"
							})).To(Succeed())
						})

						It("returns empty last operation", func() {
							Expect(err).NotTo(HaveOccurred())
							Expect(lastOp).To(Equal(domain.LastOperation{}))
						})
					})
				})
			})
		})

		Describe("unknown last operation", func() {
			BeforeEach(func() {
				details.OperationData = "unknown"
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("unknown operation")))
			})
		})
	})

	Describe("Bind", func() {
		var (
			bindingDetails domain.BindDetails
			bindingID      string
			instanceID     string
			binding        domain.Binding
			btpBinding     *btpv1.ServiceBinding
			err            error
		)

		BeforeEach(func() {
			bindingID = uuid.NewString()
			instanceID = uuid.NewString()

			btpBinding = &btpv1.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingID,
					Namespace: resourceNamespace,
				},
			}
			bindingDetails = domain.BindDetails{}
		})

		JustBeforeEach(func() {
			binding, err = broker.Bind(ctx, instanceID, bindingID, bindingDetails, true)
		})

		It("creates a btp binding", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(binding).To(Equal(domain.Binding{
				IsAsync:       true,
				OperationData: "bind-" + bindingID,
			}))

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(btpBinding), btpBinding)).To(Succeed())
			Expect(btpBinding.Spec.ServiceInstanceName).To(Equal(instanceID))
		})

		When("binding creation operation succeds", func() {
			var secret *corev1.Secret
			BeforeEach(func() {
				_, _ = broker.Bind(ctx, instanceID, bindingID, bindingDetails, true)

				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: resourceNamespace,
						Name:      btpBinding.Name,
					},
					StringData: map[string]string{
						"creds-key": "creds-value",
					},
				}
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())

				credentialsDecoder.DecodeBindingSecretDataReturns(map[string]string{
					"decoded-cred-key": "decoded-cred-value",
				}, nil)

				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(btpBinding), btpBinding)).To(Succeed())
				Expect(k8s.Patch(ctx, k8sClient, btpBinding, func() {
					btpBinding.Status = btpv1.ServiceBindingStatus{
						Ready: "True",
						Conditions: []metav1.Condition{
							{
								Type:               "Succeeded",
								Status:             metav1.ConditionTrue,
								Reason:             "Created",
								ObservedGeneration: btpBinding.Generation,
								Message:            "Service binding created successfully",
								LastTransitionTime: metav1.Now(),
							},
						},
					}
				})).To(Succeed())
			})

			It("decodes binding credentials", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(credentialsDecoder.DecodeBindingSecretDataCallCount()).To(Equal(1))
			})

			It("returns decoded secret credentials", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(binding).To(Equal(domain.Binding{
					IsAsync: false,
					Credentials: map[string]string{
						"decoded-cred-key": "decoded-cred-value",
					},
					OperationData: "bind-" + bindingID,
				}))
			})

			When("decoding binding credentials fails", func() {
				BeforeEach(func() {
					credentialsDecoder.DecodeBindingSecretDataReturns(nil, errors.New("decoding failed"))
				})

				It("returns an error", func() {
					Expect(err).To(MatchError(ContainSubstring("decoding failed")))
				})
			})
		})
	})

	Describe("Unbind", func() {
		var (
			btpServiceBinding *btpv1.ServiceBinding
			bindingID         string
			instanceID        string
			operation         domain.UnbindSpec
		)
		BeforeEach(func() {
			bindingID = uuid.NewString()
			instanceID = uuid.NewString()
			btpServiceBinding = &btpv1.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingID,
					Namespace: resourceNamespace,
				},
				Spec: btpv1.ServiceBindingSpec{
					ServiceInstanceName: instanceID,
				},
			}

			Expect(k8sClient.Create(ctx, btpServiceBinding)).To(Succeed())
		})

		JustBeforeEach(func() {
			var err error
			operation, err = broker.Unbind(ctx, instanceID, bindingID, domain.UnbindDetails{ServiceID: instanceID, PlanID: "plan-id"}, false)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the btp binding", func() {
			Consistently(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(btpServiceBinding), btpServiceBinding)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
		})

		It("returns async unbind operation", func() {
			Expect(operation).To(Equal(domain.UnbindSpec{
				IsAsync:       true,
				OperationData: "unbind-" + bindingID,
			}))
		})

		When("the binding does not exist", func() {
			BeforeEach(func() {
				bindingID = uuid.NewString()
			})

			It("still succeeds and returns async unbind operation", func() {
				Expect(operation).To(Equal(domain.UnbindSpec{
					IsAsync:       true,
					OperationData: "unbind-" + bindingID,
				}))
			})
		})
	})

	Describe("LastBindingOperation", func() {
		var (
			instanceID string
			bindingID  string
			details    domain.PollDetails
			lastOp     domain.LastOperation
			err        error
			btpBinding *btpv1.ServiceBinding
		)

		BeforeEach(func() {
			instanceID = uuid.NewString()
			bindingID = uuid.NewString()

			btpBinding = &btpv1.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingID,
					Namespace: resourceNamespace,
				},
				Spec: btpv1.ServiceBindingSpec{
					ServiceInstanceName: instanceID,
				},
			}
			details = domain.PollDetails{}

			Expect(k8sClient.Create(ctx, btpBinding)).To(Succeed())
		})

		JustBeforeEach(func() {
			lastOp, err = broker.LastBindingOperation(ctx, instanceID, bindingID, details)
		})

		Describe("bind last operation", func() {
			BeforeEach(func() {
				Expect(k8s.Patch(ctx, k8sClient, btpBinding, func() {
					btpBinding.Status = btpv1.ServiceBindingStatus{
						Ready:         "True",
						OperationType: "create",
						Conditions: []metav1.Condition{
							{
								Type:               "Succeeded",
								Status:             metav1.ConditionFalse,
								Reason:             "Created",
								ObservedGeneration: btpBinding.Generation,
								LastTransitionTime: metav1.Now(),
							},
						},
					}
				})).To(Succeed())
				details.OperationData = "bind-" + bindingID
			})

			It("returns the last operation state", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(lastOp.State).To(Equal(domain.InProgress))
			})

			When("the binding operation has succeeded", func() {
				BeforeEach(func() {
					Expect(k8s.Patch(ctx, k8sClient, btpBinding, func() {
						btpBinding.Status = btpv1.ServiceBindingStatus{
							Ready: "True",
							Conditions: []metav1.Condition{
								{
									Type:               "Succeeded",
									Status:             metav1.ConditionTrue,
									Reason:             "Created",
									ObservedGeneration: btpBinding.Generation,
									Message:            "Service binding created successfully",
									LastTransitionTime: metav1.Now(),
								},
							},
						}
					})).To(Succeed())
				})

				It("returns binding succeess last operation response", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(lastOp.State).To(Equal(domain.Succeeded))
				})
			})

			When("binding creation is failed", func() {
				BeforeEach(func() {
					Expect(k8s.Patch(ctx, k8sClient, btpBinding, func() {
						btpBinding.Status = btpv1.ServiceBindingStatus{
							Ready: "True",
							Conditions: []metav1.Condition{
								{
									Type:               "Succeeded",
									Status:             metav1.ConditionFalse,
									Reason:             "CreateFailed",
									ObservedGeneration: btpBinding.Generation,
									LastTransitionTime: metav1.Now(),
								},
								{
									Type:               "Failed",
									Status:             metav1.ConditionTrue,
									Reason:             "CreateFailed",
									ObservedGeneration: btpBinding.Generation,
									Message:            "Bind failed",
									LastTransitionTime: metav1.Now(),
								},
							},
						}
					})).To(Succeed())
				})

				It("returns the last operation state as failed", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(lastOp).To(Equal(domain.LastOperation{
						State:       domain.Failed,
						Description: "Bind failed",
					}))
				})
			})

			When("binding status is not the latest", func() {
				BeforeEach(func() {
					Expect(k8s.Patch(ctx, k8sClient, btpBinding, func() {
						btpBinding.Status = btpv1.ServiceBindingStatus{
							Ready:         "True",
							OperationType: "create",
							Conditions: []metav1.Condition{
								{
									Type:               "Succeeded",
									Status:             metav1.ConditionTrue,
									Reason:             "Created",
									ObservedGeneration: btpBinding.Generation - 1,
									LastTransitionTime: metav1.Now(),
								},
							},
						}
					})).To(Succeed())
				})

				It("returns the last operation state as in progress", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(lastOp.State).To(Equal(domain.InProgress))
				})

				When("operation type is not create", func() {
					BeforeEach(func() {
						Expect(k8s.Patch(ctx, k8sClient, btpBinding, func() {
							btpBinding.Status.OperationType = "update"
						})).To(Succeed())
					})

					It("returns empty last operation", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(lastOp).To(Equal(domain.LastOperation{}))
					})
				})
			})

			When("the binding does not exist", func() {
				BeforeEach(func() {
					bindingID = uuid.NewString()
				})

				It("returns a not found err", func() {
					Expect(err).To(MatchError(ContainSubstring("binding does not exist")))
				})
			})
		})

		Describe("unbind last operation", func() {
			BeforeEach(func() {
				details.OperationData = "unbind-" + bindingID

				Expect(k8s.Patch(ctx, k8sClient, btpBinding, func() {
					btpBinding.Status = btpv1.ServiceBindingStatus{
						Ready:         "True",
						OperationType: "delete",
						Conditions: []metav1.Condition{
							{
								Type:               "Succeeded",
								Status:             metav1.ConditionFalse,
								Reason:             "Deleted",
								ObservedGeneration: btpBinding.Generation,
								LastTransitionTime: metav1.Now(),
							},
						},
					}
				})).To(Succeed())
			})

			It("returns unbind in progress last operation response", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(lastOp.State).To(Equal(domain.InProgress))
			})

			When("the binding is gone", func() {
				BeforeEach(func() {
					bindingID = uuid.NewString()
				})

				It("returns a succeeded response", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(lastOp.State).To(Equal(domain.Succeeded))
				})
			})

			When("unbind failed", func() {
				BeforeEach(func() {
					Expect(k8s.Patch(ctx, k8sClient, btpBinding, func() {
						btpBinding.Status = btpv1.ServiceBindingStatus{
							Ready:         "True",
							OperationType: "delete",
							Conditions: []metav1.Condition{
								{
									Type:               "Failed",
									Status:             metav1.ConditionTrue,
									Reason:             "Failed",
									ObservedGeneration: btpBinding.Generation,
									Message:            "Unbind failed",
									LastTransitionTime: metav1.Now(),
								},
							},
						}
					})).To(Succeed())
				})

				It("returns a failed response", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(lastOp).To(Equal(domain.LastOperation{
						State:       domain.Failed,
						Description: "Unbind failed",
					}))
				})

				When("binding status is not the latest", func() {
					BeforeEach(func() {
						Expect(k8s.Patch(ctx, k8sClient, btpBinding, func() {
							btpBinding.Status = btpv1.ServiceBindingStatus{
								Ready:         "True",
								OperationType: "delete",
								Conditions: []metav1.Condition{
									{
										Type:               "Succeeded",
										Status:             metav1.ConditionTrue,
										Reason:             "Deleted",
										ObservedGeneration: btpBinding.Generation - 1,
										LastTransitionTime: metav1.Now(),
									},
								},
							}
						})).To(Succeed())
					})

					It("returns the last operation state as in progress", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(lastOp.State).To(Equal(domain.InProgress))
					})

					When("operation type is not delete", func() {
						BeforeEach(func() {
							Expect(k8s.Patch(ctx, k8sClient, btpBinding, func() {
								btpBinding.Status.OperationType = "update"
							})).To(Succeed())
						})

						It("returns empty last operation", func() {
							Expect(err).NotTo(HaveOccurred())
							Expect(lastOp).To(Equal(domain.LastOperation{}))
						})
					})
				})
			})
		})

		Describe("unknown last operation", func() {
			BeforeEach(func() {
				details.OperationData = "unknown"
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("unknown operation")))
			})
		})
	})
})
